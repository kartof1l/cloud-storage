package oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
)

// YandexEndpoint - эндпоинты Yandex OAuth
var YandexEndpoint = oauth2.Endpoint{
	AuthURL:  "https://oauth.yandex.ru/authorize",
	TokenURL: "https://oauth.yandex.ru/token",
}

type YandexProvider struct {
	*BaseProvider
}

func NewYandexProvider(clientID, clientSecret, redirectURL string) *YandexProvider {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"login:email", "login:info", "login:avatar"},
		Endpoint:     YandexEndpoint,
	}

	return &YandexProvider{
		BaseProvider: &BaseProvider{
			config: config,
			name:   "yandex",
		},
	}
}

func (p *YandexProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := p.config.Client(ctx, token)

	resp, err := client.Get("https://login.yandex.ru/info?format=json")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		ID            string `json:"id"`
		Login         string `json:"login"`
		Email         string `json:"default_email"`
		RealName      string `json:"real_name"`
		FirstName     string `json:"first_name"`
		LastName      string `json:"last_name"`
		DisplayName   string `json:"display_name"`
		DefaultAvatar string `json:"default_avatar_id"`
		IsAvatarEmpty bool   `json:"is_avatar_empty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %v", err)
	}

	avatarURL := ""
	if !data.IsAvatarEmpty {
		avatarURL = fmt.Sprintf("https://avatars.yandex.net/get-yapic/%s/islands-200", data.DefaultAvatar)
	}

	return &UserInfo{
		Provider:   p.Name(),
		ProviderID: data.ID,
		Email:      data.Email,
		FirstName:  data.FirstName,
		LastName:   data.LastName,
		FullName:   data.RealName,
		AvatarURL:  avatarURL,
	}, nil
}

func (p *YandexProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *YandexProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *YandexProvider) Config() *oauth2.Config {
	return p.config
}

func (p *YandexProvider) Name() string {
	return p.name
}
