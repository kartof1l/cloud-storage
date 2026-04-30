package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	ServerPort   string
	JWTSecret    string
	JWTExpiresIn time.Duration

	// Database
	DBHost      string
	DBPort      int
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	DatabaseURL string

	// Storage
	UploadPath  string
	MaxFileSize int64

	// Library Admins
	LibraryAdmins []string
}

func Load() (*Config, error) {
	// Загружаем .env файл
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Парсим список администраторов
	adminsStr := getEnv("LIBRARY_ADMINS", "")
	var admins []string
	if adminsStr != "" {
		admins = strings.Split(adminsStr, ",")
		for i := range admins {
			admins[i] = strings.TrimSpace(admins[i])
		}
	}

	// Для Render - если ENV=production, то используем SSL
	dbSSLMode := getEnv("DB_SSLMODE", "disable")
	if os.Getenv("ENV") == "production" && dbSSLMode == "disable" {
		dbSSLMode = "require"
		log.Println("Production mode: forcing SSL mode 'require'")
	}

	config := &Config{
		// Server
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		JWTSecret:    getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		JWTExpiresIn: getEnvAsDuration("JWT_EXPIRES_IN", 24*time.Hour),

		// Database
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnvAsInt("DB_PORT", 5432),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPassword:  getEnv("DB_PASSWORD", "password"),
		DBName:      getEnv("DB_NAME", "cloud_storage"),
		DBSSLMode:   dbSSLMode,
		DatabaseURL: getEnv("DATABASE_URL", ""),

		// Storage
		UploadPath:  getEnv("UPLOAD_PATH", "./uploads"),
		MaxFileSize: getEnvAsInt64("MAX_FILE_SIZE", 100*1024*1024),

		// Library Admins
		LibraryAdmins: admins,
	}

	// Создаем директорию для загрузок если её нет
	if err := os.MkdirAll(config.UploadPath, 0755); err != nil {
		return nil, err
	}

	return config, nil
}

// Вспомогательные функции
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
