package subscription

import (
	"context"

	"github.com/ldm2060/axonhub/llm/httpclient"
	"github.com/ldm2060/axonhub/llm/oauth"
)

var DefaultTokenURLs = oauth.OAuthUrls{
	AuthorizeUrl: AuthorizeURL,
	TokenUrl:     TokenURL,
}

type TokenProviderParams struct {
	Credentials *oauth.OAuthCredentials
	HTTPClient  *httpclient.HttpClient
	OnRefreshed func(ctx context.Context, refreshed *oauth.OAuthCredentials) error
}

func NewTokenProvider(params TokenProviderParams) *oauth.TokenProvider {
	return oauth.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: params.Credentials,
		HTTPClient:  params.HTTPClient,
		OAuthUrls:   DefaultTokenURLs,
		UserAgent:   CLIUserAgent,
		OnRefreshed: params.OnRefreshed,
	})
}
