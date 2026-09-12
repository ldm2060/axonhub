// Client Request Signing V4 — mirrors the ZCode desktop client
// (reverse engineered from the 3.10.2 app.asar and cross-checked against
// zcode-relay's working implementation).
//
// The z.ai coding-plan inference endpoints (zcode.z.ai /api/v1/ultra/...)
// reject plain Bearer-JWT traffic with 401/3007. They require the desktop
// client's attestation: an Ed25519 private key obtained through a signed
// handshake at the provider origin (open.bigmodel.cn/api/paas/c1f3a7e2/v2/client,
// authenticated with the two-part coding-plan API key), plus a proof-of-work
// and a per-request Ed25519 signature over
// "apiKeyId\nts\nappVersion\nsessionId\nnonce".
//
// The handshake/decryption chain, all from the client bundle:
//
//	handshake sig = base64(HMAC-SHA256(HKDF-SHA256(secret, salt=WD_CLIENT_SIGN_KDF_SALT,
//	                                                   info=getSignKey_hmac),
//	                                  "get_sign_key\n{apiKeyId}\n{ts}\n{nonce}"))
//	privateCipher = base64(IV(12) || AES-256-GCM(pkcs8Ed25519)),
//	AES key       = HKDF-SHA256(secret, salt=WD_CLIENT_SIGN_KDF_SALT, info=ed25519_priv),
//	AAD           = apiKeyId
//	pow           = first nonce12+counter(8 hex) whose SHA-256 hex digest starts
//	                 with powBits zero bits, seeded by
//	                 SHA-256("apiKeyId\nzcode\nsessionId\nts") hex[:32]

package zcode

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ldm2060/axonhub/llm/httpclient"
)

const (
	// AppVersion is the ZCode desktop client version the signer impersonates;
	// it is stamped into X-Client-Version and must match the User-Agent the
	// anthropic outbound sends.
	AppVersion = "3.11.2"

	// SigningAppID is the X-App-Id the client sends with signed requests.
	SigningAppID = "zcode"

	kdfSalt        = "WD_CLIENT_SIGN_KDF_SALT"
	kdfInfoHMAC    = "getSignKey_hmac"
	kdfInfoEd25519 = "ed25519_priv"

	handshakeMethod = "get_sign_key"
	// handshakePath is resolved against the provider origin (open.bigmodel.cn),
	// NOT zcode.z.ai — the client builds it from the provider's baseURL.
	handshakePath = "/api/paas/c1f3a7e2/v2/client"

	powBits       = 8
	nonceBytes    = 16
	powNonceBytes = 12

	handshakeTimeout = 10 * time.Second

	verifySignatureInvalid = "VERIFY_SIGNATURE_INVALID" //nolint:gosec // protocol constant, not a credential
	verifyAPIKeyExpired    = "VERIFY_APIKEY_EXPIRED"    //nolint:gosec // protocol constant, not a credential
)

// hkdfSHA256 derives 32 bytes of key material (extract-then-expand, RFC 5869).
func hkdfSHA256(secret string, info string) []byte {
	prk := hmac.New(sha256.New, []byte(kdfSalt))
	prk.Write([]byte(secret))
	out := make([]byte, 0, 32)
	var t []byte
	for i := byte(1); len(out) < 32; i++ {
		h := hmac.New(sha256.New, prk.Sum(nil))
		h.Write(append(append(append([]byte{}, t...), []byte(info)...), i))
		t = h.Sum(nil)
		out = append(out, t...)
	}
	return out[:32]
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// RequestSigner holds the coding-plan signing state for one two-part API key:
// the Ed25519 key obtained from the handshake (cached until a VERIFY rejection
// invalidates it) and the session id bound into every signature.
type RequestSigner struct {
	httpClient   *httpclient.HttpClient
	apiKeyID     string
	apiKeySecret string
	origin       string
	sessionID    string

	mu      sync.Mutex
	privKey ed25519.PrivateKey
	bypass  bool
}

// SignerParams configures a RequestSigner.
type SignerParams struct {
	HTTPClient *httpclient.HttpClient
	APIKeyID   string
	// APIKeySecret is the secret half of the two-part coding-plan key.
	APIKeySecret string
	// Origin is the handshake origin — the provider API host
	// (https://open.bigmodel.cn). Empty defaults to BigModelAPIOrigin.
	Origin string
	// SessionID overrides the generated session id (mainly for tests).
	SessionID string
}

// NewRequestSigner creates a signer for a two-part coding-plan key.
func NewRequestSigner(params SignerParams) *RequestSigner {
	origin := params.Origin
	if origin == "" {
		origin = BigModelAPIOrigin
	}
	sessionID := params.SessionID
	if sessionID == "" {
		sessionID = randomHex(16)
	}
	return &RequestSigner{
		httpClient:   params.HTTPClient,
		apiKeyID:     params.APIKeyID,
		apiKeySecret: params.APIKeySecret,
		origin:       strings.TrimSuffix(origin, "/"),
		sessionID:    sessionID,
		mu:           sync.Mutex{},
		privKey:      nil,
		bypass:       false,
	}
}

// handshakeEnvelope is the signed-handshake response.
type handshakeEnvelope struct {
	Code *int   `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		PrivateCipher string `json:"privateCipher"`
	} `json:"data"`
}

// handshake performs the key-enrollment handshake and returns the decrypted
// Ed25519 private key.
func (s *RequestSigner) handshake(ctx context.Context) (ed25519.PrivateKey, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := randomHex(nonceBytes)

	mac := hmac.New(sha256.New, hkdfSHA256(s.apiKeySecret, kdfInfoHMAC))
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s", handshakeMethod, s.apiKeyID, ts, nonce)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	body, err := json.Marshal(map[string]string{
		"apiKey": s.apiKeyID + "." + s.apiKeySecret,
		"nonce":  nonce,
		"sig":    sig,
		"ts":     ts,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal handshake request: %w", err)
	}

	hctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	cred := s.apiKeyID + "." + s.apiKeySecret
	req := &httpclient.Request{ //nolint:exhaustruct_v5 // only the request plumbing matters here.
		Method: http.MethodPost,
		URL:    s.origin + handshakePath,
		Headers: http.Header{
			"Authorization": []string{cred},
			"Content-Type":  []string{"application/json"},
			"User-Agent":    []string{"ZCode/" + AppVersion},
			"Http-Referer":  []string{"https://zcode.z.ai"},
			"X-Title":       []string{"Z Code@electron"},
		},
		Body: body,
	}

	resp, err := s.httpClient.Do(hctx, req)
	if err != nil {
		return nil, fmt.Errorf("handshake request: %w", err)
	}

	var envelope handshakeEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return nil, fmt.Errorf("decode handshake response: %w", err)
	}
	if envelope.Code == nil || *envelope.Code != http.StatusOK {
		return nil, fmt.Errorf("handshake rejected: code=%v msg=%s", envelope.Code, envelope.Msg)
	}

	return decryptSigningPrivateKey(s.apiKeyID, s.apiKeySecret, envelope.Data.PrivateCipher)
}

// decryptSigningPrivateKey decrypts the AES-256-GCM privateCipher into the
// PKCS#8 Ed25519 private key.
func decryptSigningPrivateKey(apiKeyID, secret, privateCipher string) (ed25519.PrivateKey, error) {
	cipherBytes, err := base64.StdEncoding.DecodeString(privateCipher)
	if err != nil {
		return nil, fmt.Errorf("decode privateCipher: %w", err)
	}
	if len(cipherBytes) <= 12+16 {
		return nil, errors.New("privateCipher is too short")
	}

	block, err := aes.NewCipher(hkdfSHA256(secret, kdfInfoEd25519))
	if err != nil {
		return nil, fmt.Errorf("init aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}

	plain, err := gcm.Open(nil, cipherBytes[:12], cipherBytes[12:], []byte(apiKeyID))
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}

	pkcs8, err := base64.StdEncoding.DecodeString(string(plain))
	if err != nil {
		return nil, fmt.Errorf("decode pkcs8: %w", err)
	}

	key, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8: %w", err)
	}
	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unexpected private key type %T", key)
	}
	return edKey, nil
}

// ensurePrivateKey returns the cached signing key, running the handshake when
// none is cached (or after an invalidation).
func (s *RequestSigner) ensurePrivateKey(ctx context.Context) (ed25519.PrivateKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.privKey != nil {
		return s.privKey, nil
	}
	key, err := s.handshake(ctx)
	if err != nil {
		return nil, err
	}
	s.privKey = key
	return key, nil
}

func (s *RequestSigner) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.privKey = nil
}

// solveProofOfWork finds a nonce+counter whose digest carries powBits leading
// zero bits over the seeded challenge.
func solveProofOfWork(apiKeyID, sessionID, ts string) (string, error) {
	seedDigest := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s\n%s\n%s", apiKeyID, SigningAppID, sessionID, ts)))
	seed := hex.EncodeToString(seedDigest[:])[:32]
	prefix := randomHex(powNonceBytes)

	target := strings.Repeat("0", powBits/4)
	for counter := range 4294967296 {
		candidate := prefix + fmt.Sprintf("%08x", counter)
		digest := sha256.Sum256([]byte(seed + "\n" + candidate))
		if strings.HasPrefix(hex.EncodeToString(digest[:]), target) {
			return candidate, nil
		}
	}
	return "", errors.New("unable to solve client request proof of work")
}

// SignedHeaders computes the per-request X-Client-* header set.
func (s *RequestSigner) SignedHeaders(ctx context.Context) (http.Header, error) {
	key, err := s.ensurePrivateKey(ctx)
	if err != nil {
		return nil, err
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := randomHex(nonceBytes)

	pow, err := solveProofOfWork(s.apiKeyID, s.sessionID, ts)
	if err != nil {
		return nil, err
	}

	sig := ed25519.Sign(key, []byte(fmt.Sprintf("%s\n%s\n%s\n%s\n%s", s.apiKeyID, ts, AppVersion, s.sessionID, nonce)))

	return http.Header{
		"X-Client-Ts":      []string{ts},
		"X-Client-Version": []string{AppVersion},
		"X-Client-Sig":     []string{base64.StdEncoding.EncodeToString(sig)},
		"X-Session-Id":     []string{s.sessionID},
		"X-Client-Nonce":   []string{nonce},
		"X-App-Id":         []string{SigningAppID},
		"X-Client-Pow":     []string{pow},
	}, nil
}

// isVerifyFailure reports whether the response is a signing rejection the
// client retries on (401 with VERIFY_SIGNATURE_INVALID / VERIFY_APIKEY_EXPIRED).
func isVerifyFailure(status int, body []byte) bool {
	if status != http.StatusUnauthorized {
		return false
	}
	return bytes.Contains(body, []byte(verifySignatureInvalid)) ||
		bytes.Contains(body, []byte(verifyAPIKeyExpired))
}

// SigningRoundTripper signs outgoing coding-plan requests and applies the
// server-controlled endpoint routing. It mirrors the client's retry ladder:
// signed → on 401 VERIFY re-handshake and retry once → bypass signing.
type SigningRoundTripper struct {
	inner   http.RoundTripper
	signer  *RequestSigner
	routing *EndpointRouting
}

// NewSigningRoundTripper wraps an existing transport.
func NewSigningRoundTripper(inner http.RoundTripper, signer *RequestSigner, routing *EndpointRouting) *SigningRoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return &SigningRoundTripper{inner: inner, signer: signer, routing: routing}
}

// NewSigningClient clones the base HttpClient with a signing transport. The
// base client's transport (including any proxy) becomes the inner hop.
func NewSigningClient(base *httpclient.HttpClient, signer *RequestSigner, routing *EndpointRouting) *httpclient.HttpClient {
	native := base.GetNativeClient()
	cloned := &http.Client{
		Transport:     NewSigningRoundTripper(native.Transport, signer, routing),
		CheckRedirect: native.CheckRedirect,
		Jar:           native.Jar,
		Timeout:       native.Timeout,
	}
	return httpclient.NewHttpClientWithClient(cloned)
}

func (t *SigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Never sign the handshake itself; on a bypassed signer send unsigned.
	if strings.HasSuffix(req.URL.Path, handshakePath) || t.signer.isBypassed() {
		return t.send(req, nil)
	}

	routed := req
	if t.routing != nil {
		if target := t.routing.resolve(req.URL); target != nil {
			clone := req.Clone(req.Context())
			clone.URL = target
			clone.Host = target.Host
			routed = clone
		}
	}

	headers, err := t.signer.SignedHeaders(req.Context())
	if err != nil {
		// Fail open like the client: handshake/PoW failure sends unsigned.
		return t.send(routed, nil)
	}

	resp, err := t.send(routed, headers)
	if err != nil {
		return nil, err
	}
	if !isVerifyFailure(resp.StatusCode, readBodyForRetry(resp)) {
		return resp, nil
	}

	// One retry ladder: invalidate the key, re-handshake, re-sign.
	t.signer.invalidate()
	retryHeaders, err := t.signer.SignedHeaders(req.Context())
	if err != nil {
		return resp, nil //nolint:nilerr // the original verify-failed response is the more useful outcome
	}
	resp2, err := t.send(routed, retryHeaders)
	if err != nil {
		return nil, err
	}
	if isVerifyFailure(resp2.StatusCode, readBodyForRetry(resp2)) {
		t.signer.setBypass()
	}
	return resp2, nil
}

// send dispatches with the signing headers (if any) merged in. The header
// merge happens on a clone so the pooled request stays untouched.
func (t *SigningRoundTripper) send(req *http.Request, signing http.Header) (*http.Response, error) {
	if len(signing) == 0 {
		return t.inner.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	maps.Copy(clone.Header, signing)
	return t.inner.RoundTrip(clone)
}

func (s *RequestSigner) isBypassed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bypass
}

func (s *RequestSigner) setBypass() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bypass = true
}

// readBodyForRetry buffers (and restores) the response body so a verify
// rejection can be inspected without consuming it for the caller. Verify
// rejections only ever arrive on 401, so every other response — notably
// streaming SSE bodies — is left untouched: buffering would block until the
// limit is reached and replace the body with just those bytes, silently
// truncating streams larger than the limit (usage is lost with them). The
// buffered prefix is always re-attached ahead of the remaining stream, so
// nothing is dropped even when the read fails partway. It returns nil when
// the body was not buffered or could not be read.
func readBodyForRetry(resp *http.Response) []byte {
	if resp.Body == nil || resp.StatusCode != http.StatusUnauthorized {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), resp.Body))
	if err != nil {
		return nil
	}
	return body
}
