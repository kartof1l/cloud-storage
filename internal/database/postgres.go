package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"

	"cloud-storage-go/internal/config"
)

func Connect(cfg *config.Config) (*sql.DB, error) {
	var connStr string

	if cfg.DatabaseURL != "" {
		connStr = cfg.DatabaseURL
	} else {
		// Определяем sslmode
		sslmode := cfg.DBSSLMode
		if sslmode == "" || sslmode == "disable" {
			// На Render всегда используем require
			if os.Getenv("ENV") == "production" {
				sslmode = "require"
				log.Println("Using SSL mode: require (production)")
			} else {
				sslmode = "disable"
			}
		}

		connStr = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, sslmode,
		)
	}

	log.Printf("Connecting to database with SSL mode: %s",
		func() string {
			if cfg.DBSSLMode != "" {
				return cfg.DBSSLMode
			}
			return "auto"
		}())

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	log.Println("Database connected successfully!")

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

func CreateTables(db *sql.DB) error {
	queries := []string{
		// Таблица users
		`
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			avatar_url VARCHAR(512),
			is_email_verified BOOLEAN DEFAULT FALSE,
			verification_code VARCHAR(10),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		`,

		// Таблица files с folder_size
		`
		CREATE TABLE IF NOT EXISTS files (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			original_name VARCHAR(255) NOT NULL,
			path VARCHAR(1024),
			size BIGINT DEFAULT 0,
			folder_size BIGINT DEFAULT 0,
			mime_type VARCHAR(255),
			is_folder BOOLEAN DEFAULT FALSE,
			parent_folder_id UUID REFERENCES files(id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		`,

		// Индексы для files
		`CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_files_parent_folder ON files(parent_folder_id);`,
		`CREATE INDEX IF NOT EXISTS idx_files_is_folder ON files(is_folder);`,

		// Таблица oauth_accounts
		`
		CREATE TABLE IF NOT EXISTS oauth_accounts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider VARCHAR(50) NOT NULL,
			provider_user_id VARCHAR(255) NOT NULL,
			access_token TEXT,
			refresh_token TEXT,
			token_expiry TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(provider, provider_user_id)
		);
		`,

		// Индексы для oauth_accounts
		`CREATE INDEX IF NOT EXISTS idx_oauth_user_id ON oauth_accounts(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_provider ON oauth_accounts(provider, provider_user_id);`,

		// Таблица library_items
		`
		CREATE TABLE IF NOT EXISTS library_items (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			mime_type VARCHAR(255),
			size BIGINT DEFAULT 0,
			path VARCHAR(1024),
			is_folder BOOLEAN DEFAULT FALSE,
			parent_id UUID REFERENCES library_items(id) ON DELETE CASCADE,
			version INT DEFAULT 1,
			created_by UUID NOT NULL REFERENCES users(id),
			updated_by UUID REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		);
		`,

		// Таблица library_versions
		`
		CREATE TABLE IF NOT EXISTS library_versions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			item_id UUID NOT NULL REFERENCES library_items(id) ON DELETE CASCADE,
			version INT NOT NULL,
			size BIGINT,
			path VARCHAR(1024),
			created_by UUID NOT NULL REFERENCES users(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		`,

		// Таблица library_admins
		`
		CREATE TABLE IF NOT EXISTS library_admins (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) NOT NULL UNIQUE,
			user_id UUID REFERENCES users(id),
			added_by UUID NOT NULL REFERENCES users(id),
			added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		`,

		// Таблица audit_logs
		`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			user_email VARCHAR(255) NOT NULL,
			action VARCHAR(50) NOT NULL,
			entity_type VARCHAR(50) NOT NULL,
			entity_id UUID,
			entity_name VARCHAR(255),
			details TEXT,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		`,

		// Индексы для библиотеки
		`CREATE INDEX IF NOT EXISTS idx_library_items_parent ON library_items(parent_id);`,
		`CREATE INDEX IF NOT EXISTS idx_library_items_name ON library_items(name);`,
		`CREATE INDEX IF NOT EXISTS idx_library_items_deleted ON library_items(deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_library_versions_item ON library_versions(item_id);`,
		`CREATE INDEX IF NOT EXISTS idx_library_admins_email ON library_admins(email);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("error executing query: %v\nQuery: %s", err, query)
		}
	}

	return nil
}
