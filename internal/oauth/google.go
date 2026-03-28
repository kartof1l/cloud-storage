package oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	*BaseProvider
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &GoogleProvider{
		BaseProvider: &BaseProvider{
			config: config,
			name:   "google",
		},
	}
}

func (p *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := p.config.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %v", err)
	}

	return &UserInfo{
		Provider:   p.Name(),
		ProviderID: data.ID,
		Email:      data.Email,
		FirstName:  data.GivenName,
		LastName:   data.FamilyName,
		FullName:   data.Name,
		AvatarURL:  data.Picture,
	}, nil
}

func (p *GoogleProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *GoogleProvider) Config() *oauth2.Config {
	return p.config
}

func (p *GoogleProvider) Name() string {
	return p.name
}
