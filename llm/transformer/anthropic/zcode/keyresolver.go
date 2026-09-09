// Coding-plan key resolution — mirrors the ZCode client's post-OAuth key
// provisioning. The OAuth flow yields a provider access token, but the
// coding-plan inference endpoints (and the signing handshake) authenticate
// with a two-part API key {apiKey}.{secretKey} owned by the BigModel account.
// The resolution chain, verified live against bigmodel.cn:
//
//	GET  /api/biz/customer/getCustomerInfo          → default org + project
//	GET  /api/biz/v1/organization/{org}/projects/{proj}/api_keys
//	POST .../api_keys {"name":"zcode-api-key"}       → create when missing
//	GET  .../api_keys/copy/{apiKey}                  → the real secretKey
//	                                                 (list/create return it masked)

package zcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

const (
	// BigModelAPIOrigin hosts the biz API and the signing handshake.
	//nolint:gosec // false alert.
	BigModelAPIOrigin = "https://open.bigmodel.cn"
	// BigModelBizOrigin hosts the organization/project biz API.
	//nolint:gosec // false alert.
	BigModelBizOrigin = "https://bigmodel.cn"

	codingPlanKeyName    = "zcode-api-key"
	defaultOrgMarker     = "默认机构"
	defaultProjectMarker = "默认项目"
	keyResolutionTimeout = 15 * time.Second
)

type bizEnvelope struct {
	Code *int            `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func bizOK(code int) bool { return code == 0 || code == http.StatusOK }

// requestBiz performs an authorized biz API call and decodes data.
func requestBiz(ctx context.Context, client *httpclient.HttpClient, method, rawURL, authorization string, body []byte, out any) error {
	req := &httpclient.Request{ //nolint:exhaustruct_v5 // only the request plumbing matters here.
		Method: method,
		URL:    rawURL,
		Headers: http.Header{
			"Authorization": []string{authorization},
			"Content-Type":  []string{"application/json"},
			"User-Agent":    []string{"ZCode/" + AppVersion},
			"Http-Referer":  []string{"https://zcode.z.ai"},
		},
	}
	if body != nil {
		req.Body = body
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("biz request %s %s: %w", method, rawURL, err)
	}

	var envelope bizEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return fmt.Errorf("decode biz response %s: %w", rawURL, err)
	}
	if envelope.Code == nil || !bizOK(*envelope.Code) {
		return fmt.Errorf("biz request %s rejected: code=%v msg=%s", rawURL, envelope.Code, envelope.Msg)
	}
	if out == nil || len(envelope.Data) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

type bizOrganization struct {
	OrganizationName string `json:"organizationName"`
	Name             string `json:"name"`
	OrganizationID   string `json:"organizationId"`
	OrgID            string `json:"orgId"`
	ID               string `json:"id"`
	Projects         []struct {
		ProjectName string `json:"projectName"`
		Name        string `json:"name"`
		ProjectID   string `json:"projectId"`
		ID          string `json:"id"`
	} `json:"projects"`
}

func (o bizOrganization) name() string {
	if o.OrganizationName != "" {
		return o.OrganizationName
	}
	return o.Name
}

func (o bizOrganization) id() string {
	if o.OrganizationID != "" {
		return o.OrganizationID
	}
	if o.OrgID != "" {
		return o.OrgID
	}
	return o.ID
}

// unmarshalOrganizations decodes the getCustomerInfo data payload. The live
// API has been observed returning an object (with the org list nested under
// various keys, or a single organization directly) as well as a bare array —
// accept every shape instead of failing the whole provisioning chain.
func unmarshalOrganizations(raw json.RawMessage, rawURL string) ([]bizOrganization, error) {
	trimmed := bytes.TrimSpace(raw)

	var list []bizOrganization
	if err := json.Unmarshal(trimmed, &list); err == nil {
		return list, nil
	}

	var wrapped struct {
		Organizations []bizOrganization `json:"organizations"`
		Organization  []bizOrganization `json:"organization"`
		Orgs          []bizOrganization `json:"orgs"`
		Data          []bizOrganization `json:"data"`
		List          []bizOrganization `json:"list"`
	}
	if err := json.Unmarshal(trimmed, &wrapped); err == nil {
		for _, candidate := range [][]bizOrganization{
			wrapped.Organizations, wrapped.Organization, wrapped.Orgs, wrapped.Data, wrapped.List,
		} {
			if len(candidate) > 0 {
				return candidate, nil
			}
		}
	}

	var one bizOrganization
	if err := json.Unmarshal(trimmed, &one); err == nil && one.id() != "" {
		return []bizOrganization{one}, nil
	}

	return nil, fmt.Errorf("unrecognized getCustomerInfo data shape from %s: %s", rawURL, truncateForLog(raw))
}

func truncateForLog(raw json.RawMessage) string {
	const max = 512
	if len(raw) > max {
		return string(raw[:max]) + "...(truncated)"
	}
	return string(raw)
}

// ResolveCodingPlanKey provisions (or finds) the BigModel coding-plan API key
// and returns its two halves. authorization is the raw BigModel OAuth access
// token from the zcode exchange.
func ResolveCodingPlanKey(ctx context.Context, client *httpclient.HttpClient, authorization string) (apiKeyID, apiKeySecret string, err error) {
	ctx, cancel := context.WithTimeout(ctx, keyResolutionTimeout)
	defer cancel()

	infoURL := BigModelBizOrigin + "/api/biz/customer/getCustomerInfo"

	var raw json.RawMessage
	if err := requestBiz(ctx, client, http.MethodGet, infoURL, authorization, nil, &raw); err != nil {
		return "", "", fmt.Errorf("resolve customer info: %w", err)
	}

	orgs, err := unmarshalOrganizations(raw, infoURL)
	if err != nil {
		return "", "", fmt.Errorf("resolve customer info: %w", err)
	}

	org, project, err := defaultOrgProject(orgs)
	if err != nil {
		return "", "", err
	}

	keysURL := fmt.Sprintf("%s/api/biz/v1/organization/%s/projects/%s/api_keys", BigModelBizOrigin, url.PathEscape(org), url.PathEscape(project))

	var existing []struct {
		APIKey string `json:"apiKey"`
		Name   string `json:"name"`
	}
	// A list failure is tolerated — we fall through to create.
	_ = requestBiz(ctx, client, http.MethodGet, keysURL, authorization, nil, &existing)

	for _, k := range existing {
		if k.Name == codingPlanKeyName && k.APIKey != "" {
			apiKeyID = k.APIKey
			break
		}
	}

	if apiKeyID == "" {
		var created struct {
			APIKey string `json:"apiKey"`
		}
		if err := requestBiz(ctx, client, http.MethodPost, keysURL, authorization, []byte(fmt.Sprintf(`{"name":%q}`, codingPlanKeyName)), &created); err != nil {
			return "", "", fmt.Errorf("create coding-plan key: %w", err)
		}
		if created.APIKey == "" {
			return "", "", errors.New("create coding-plan key: response missing apiKey")
		}
		apiKeyID = created.APIKey
	}

	// The list/create endpoints return the secret masked; only copy reveals it.
	var copied struct {
		SecretKey string `json:"secretKey"`
	}
	copyURL := keysURL + "/copy/" + url.PathEscape(apiKeyID)
	if err := requestBiz(ctx, client, http.MethodGet, copyURL, authorization, nil, &copied); err != nil {
		return "", "", fmt.Errorf("copy coding-plan key secret: %w", err)
	}
	copied.SecretKey = strings.TrimSpace(copied.SecretKey)
	if copied.SecretKey == "" || strings.HasPrefix(copied.SecretKey, "*") {
		return "", "", errors.New("copy coding-plan key secret: secretKey missing or masked")
	}

	return apiKeyID, copied.SecretKey, nil
}

func defaultOrgProject(orgs []bizOrganization) (orgID, projectID string, err error) {
	if len(orgs) == 0 {
		return "", "", errors.New("no organizations found for coding-plan key")
	}
	org := orgs[0]
	for _, o := range orgs {
		if strings.Contains(o.name(), defaultOrgMarker) {
			org = o
			break
		}
	}
	if len(org.Projects) == 0 {
		return "", "", errors.New("no projects found in coding-plan organization")
	}
	project := org.Projects[0]
	for _, p := range org.Projects {
		if strings.Contains(projectName(p), defaultProjectMarker) {
			project = p
			break
		}
	}
	pid := project.ProjectID
	if pid == "" {
		pid = project.ID
	}
	if org.id() == "" || pid == "" {
		return "", "", errors.New("coding-plan organization/project id missing")
	}
	return org.id(), pid, nil
}

func projectName(p struct {
	ProjectName string `json:"projectName"`
	Name        string `json:"name"`
	ProjectID   string `json:"projectId"`
	ID          string `json:"id"`
},
) string {
	if p.ProjectName != "" {
		return p.ProjectName
	}
	return p.Name
}
