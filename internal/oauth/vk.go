package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"
)

// VKEndpoint - эндпоинты VK OAuth
var VKEndpoint = oauth2.Endpoint{
	AuthURL:  "https://oauth.vk.com/authorize",
	TokenURL: "https://oauth.vk.com/access_token",
}

type VKProvider struct {
	*BaseProvider
	apiVersion string
}

func NewVKProvider(clientID, clientSecret, redirectURL string) *VKProvider {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"email", "photos"},
		Endpoint:     VKEndpoint,
	}

	return &VKProvider{
		BaseProvider: &BaseProvider{
			config: config,
			name:   "vk",
		},
		apiVersion: "5.131",
	}
}

// VK возвращает токен вместе с email в ответе
func (p *VKProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	// VK добавляет email в ответ при обмене кода
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (p *VKProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	// Извлекаем email из токена (VK возвращает email в extra)
	email, _ := token.Extra("email").(string)

	client := p.config.Client(ctx, token)

	// Запрашиваем информацию о пользователе
	params := url.Values{}
	params.Set("access_token", token.AccessToken)
	params.Set("v", p.apiVersion)
	params.Set("fields", "first_name,last_name,photo_max_orig")

	apiURL := "https://api.vk.com/method/users.get?" + params.Encode()

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		Response []struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			PhotoMax  string `json:"photo_max_orig"`
		} `json:"response"`
		Error *struct {
			ErrorCode int    `json:"error_code"`
			ErrorMsg  string `json:"error_msg"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %v", err)
	}

	if data.Error != nil {
		return nil, fmt.Errorf("VK API error: %s", data.Error.ErrorMsg)
	}

	if len(data.Response) == 0 {
		return nil, fmt.Errorf("no user data returned")
	}

	user := data.Response[0]

	return &UserInfo{
		Provider:   p.Name(),
		ProviderID: fmt.Sprintf("%d", user.ID),
		Email:      email,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		FullName:   fmt.Sprintf("%s %s", user.FirstName, user.LastName),
		AvatarURL:  user.PhotoMax,
	}, nil
}

func (p *VKProvider) GetAuthURL(state string) string {
	// VK требует дополнительные параметры
	u, _ := url.Parse(p.config.AuthCodeURL(state))
	q := u.Query()
	q.Set("display", "page")
	q.Set("revoke", "1")
	u.RawQuery = q.Encode()
	return u.String()
}

func (p *VKProvider) Config() *oauth2.Config {
	return p.config
}

func (p *VKProvider) Name() string {
	return p.name
}
