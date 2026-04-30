package oauth

import (
	"context"

	"golang.org/x/oauth2"
)

// UserInfo - информация о пользователе от провайдера
type UserInfo struct {
	Provider   string
	ProviderID string
	Email      string
	FirstName  string
	LastName   string
	FullName   string
	AvatarURL  string
}

// Provider - интерфейс OAuth провайдера
type Provider interface {
	// Название провайдера (google, yandex, vk)
	Name() string

	// Получение URL для редиректа пользователя
	GetAuthURL(state string) string

	// Обмен кода на токен
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)

	// Получение информации о пользователе
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error)

	// Настройки провайдера
	Config() *oauth2.Config
}

// BaseProvider - базовая структура для всех провайдеров
type BaseProvider struct {
	config *oauth2.Config
	name   string
}
