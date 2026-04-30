package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"cloud-storage-go/internal/config"
	"cloud-storage-go/internal/crypto"
	"cloud-storage-go/internal/database"
	"cloud-storage-go/internal/handlers"
	"cloud-storage-go/internal/middleware"
	"cloud-storage-go/internal/oauth"
	"cloud-storage-go/internal/repository"
	"cloud-storage-go/internal/services"
	"cloud-storage-go/internal/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("❌ Failed to load config:", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}
	defer db.Close()

	if err := database.CreateTables(db); err != nil {
		log.Fatal("❌ Failed to create tables:", err)
	}
	log.Println("✅ Database connected and migrated successfully!")

	// Создание директорий
	dirs := []string{
		filepath.Join(cfg.UploadPath, "avatars"),
		filepath.Join(cfg.UploadPath, "users"),
		filepath.Join(cfg.UploadPath, "library"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("⚠️  Could not create directory %s: %v", dir, err)
		}
	}

	// Инициализация репозиториев
	userRepo := repository.NewUserRepository(db)
	fileRepo := repository.NewFileRepository(db)
	oauthRepo := repository.NewOAuthRepository(db)
	libRepo := repository.NewLibraryRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)

	// Инициализация утилит
	jwtUtil := utils.NewJWTUtil(cfg.JWTSecret, cfg.JWTExpiresIn)
	encryptionService := crypto.NewEncryptionService(cfg.JWTSecret)

	// Инициализация сервисов
	auditService := services.NewAuditService(auditRepo)
	storageService := services.NewStorageService(cfg.UploadPath)
	emailService := services.NewEmailService()
	libraryService := services.NewLibraryService(libRepo, userRepo, encryptionService, cfg.UploadPath, auditService)
	authService := services.NewAuthService(userRepo, oauthRepo, jwtUtil, emailService)
	fileService := services.NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	// Rate Limiters
	uploadLimiter := newIPRateLimiter(5, 10)
	authLimiter := newIPRateLimiter(3, 5)
	apiLimiter := newIPRateLimiter(20, 50)
	downloadLimiter := newIPRateLimiter(10, 20)

	// Инициализация администраторов
	if len(cfg.LibraryAdmins) > 0 {
		var initiatorID uuid.UUID
		err := db.QueryRow("SELECT id FROM users WHERE email = $1", cfg.LibraryAdmins[0]).Scan(&initiatorID)
		if err != nil {
			log.Printf("⚠️  Admin user %s not found, using temporary ID", cfg.LibraryAdmins[0])
			initiatorID = uuid.New()
		}
		if err := libraryService.InitAdminsFromConfig(cfg.LibraryAdmins, initiatorID); err != nil {
			log.Printf("⚠️  Failed to init admins: %v", err)
		} else {
			log.Printf("✅ Initialized %d library admins", len(cfg.LibraryAdmins))
		}
	}

	// OAuth провайдеры
	var googleProvider, yandexProvider, vkProvider oauth.Provider
	if id := os.Getenv("GOOGLE_CLIENT_ID"); id != "" {
		googleProvider = oauth.NewGoogleProvider(id, os.Getenv("GOOGLE_CLIENT_SECRET"), os.Getenv("GOOGLE_REDIRECT_URL"))
	}
	if id := os.Getenv("YANDEX_CLIENT_ID"); id != "" {
		yandexProvider = oauth.NewYandexProvider(id, os.Getenv("YANDEX_CLIENT_SECRET"), os.Getenv("YANDEX_REDIRECT_URL"))
	}
	if id := os.Getenv("VK_CLIENT_ID"); id != "" {
		vkProvider = oauth.NewVKProvider(id, os.Getenv("VK_CLIENT_SECRET"), os.Getenv("VK_REDIRECT_URL"))
	}

	// Конфигурация сессий
	sessionConfig := &middleware.SessionConfig{
		SecretKey: getEnvOrDefault("SESSION_SECRET", "change-me-in-production-"+uuid.New().String()),
		MaxAge:    86400 * 7,
		Secure:    os.Getenv("ENV") == "production",
		HttpOnly:  true,
		SameSite:  http.SameSiteLaxMode,
	}
	if os.Getenv("ENV") == "production" {
		sessionConfig.SameSite = http.SameSiteStrictMode
	}
	sessionManager := middleware.NewSessionManager(sessionConfig)

	// Хендлеры
	authHandler := handlers.NewAuthHandler(authService)
	fileHandler := handlers.NewFileHandler(fileService)
	folderHandler := handlers.NewFolderHandler(fileService)
	libraryHandler := handlers.NewLibraryHandler(libraryService)
	userHandler := handlers.NewUserHandler(userRepo, cfg.UploadPath)
	adminHandler := handlers.NewAdminHandler(userRepo, auditService, libraryService, db)
	scheduleHandler := handlers.NewScheduleHandler(scheduleRepo, userRepo)

	oauthHandler := handlers.NewOAuthHandler(
		googleProvider, yandexProvider, vkProvider,
		sessionManager, authService,
	)

	// Настройка роутера
	router := setupRouter(cfg, db, jwtUtil, sessionManager, userRepo,
		authHandler, fileHandler, folderHandler, libraryHandler,
		userHandler, adminHandler, scheduleHandler, oauthHandler,
		uploadLimiter, authLimiter, apiLimiter, downloadLimiter)

	// Запуск сервера
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.ServerPort
	}

	log.Printf("🚀 Server starting on port %s (ENV: %s)", port, getEnv())
	if err := router.Run(":" + port); err != nil {
		log.Fatal("❌ Failed to start server:", err)
	}
}

// IPRateLimiter - rate limiter с привязкой к IP
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.RWMutex
	r   rate.Limit
	b   int
}

func newIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}
	return limiter
}

func setupRouter(
	cfg *config.Config,
	db *sql.DB,
	jwtUtil *utils.JWTUtil,
	sessionManager *middleware.SessionManager,
	userRepo *repository.UserRepository,
	authHandler *handlers.AuthHandler,
	fileHandler *handlers.FileHandler,
	folderHandler *handlers.FolderHandler,
	libraryHandler *handlers.LibraryHandler,
	userHandler *handlers.UserHandler,
	adminHandler *handlers.AdminHandler,
	scheduleHandler *handlers.ScheduleHandler,
	oauthHandler *handlers.OAuthHandler,
	uploadLimiter, authLimiter, apiLimiter, downloadLimiter *IPRateLimiter,
) *gin.Engine {
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.SecurityHeadersMiddleware())

	// CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     getAllowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.Use(sessionManager.SessionMiddleware())

	// Статические файлы
	router.Static("/css", "./web/css")
	router.Static("/js", "./web/js")
	router.Static("/uploads", cfg.UploadPath)

	pages := map[string]string{
		"/":          "./web/index.html",
		"/login":     "./web/login.html",
		"/register":  "./web/register.html",
		"/admin":     "./web/admin.html",
		"/dashboard": "./web/dashboard.html",
		"/library":   "./web/library.html",
		"/verify":    "./web/verify.html",
		"/status":    "./web/status.html",
	}
	for route, file := range pages {
		router.StaticFile(route, file)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "env": getEnv()})
	})

	// API Auth routes
	auth := router.Group("/api/auth")
	{
		auth.POST("/register", rateLimitMiddleware(authLimiter), authHandler.Register)
		auth.POST("/login", rateLimitMiddleware(authLimiter), authHandler.Login)
		auth.POST("/verify", rateLimitMiddleware(authLimiter), authHandler.VerifyEmail)
		auth.POST("/resend-code", rateLimitMiddleware(authLimiter), authHandler.ResendCode)
		auth.GET("/:provider/login", oauthHandler.Login)
		auth.GET("/:provider/callback", oauthHandler.Callback)
	}

	// Protected routes
	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware(jwtUtil))
	{
		// Files
		protected.POST("/files/upload", rateLimitMiddleware(uploadLimiter), fileHandler.UploadFile)
		protected.GET("/files", rateLimitMiddleware(apiLimiter), fileHandler.ListFiles)
		protected.GET("/storage/stats", rateLimitMiddleware(apiLimiter), fileHandler.GetStorageStats)
		protected.GET("/files/:id", rateLimitMiddleware(apiLimiter), fileHandler.GetFile)
		protected.GET("/files/:id/download", rateLimitMiddleware(downloadLimiter), fileHandler.DownloadFile)
		protected.DELETE("/files/:id", rateLimitMiddleware(apiLimiter), fileHandler.DeleteFile)
		protected.PUT("/files/:id", rateLimitMiddleware(apiLimiter), fileHandler.RenameFile)
		protected.PUT("/files/:id/move", rateLimitMiddleware(apiLimiter), fileHandler.MoveFile)
		protected.POST("/files/:id/move-to-library", rateLimitMiddleware(apiLimiter), fileHandler.QuickMoveToLibrary)

		// Folders
		protected.POST("/folders", rateLimitMiddleware(apiLimiter), folderHandler.CreateFolder)
		protected.GET("/folders/:id/contents", rateLimitMiddleware(apiLimiter), folderHandler.GetFolderContents)
		protected.DELETE("/folders/:id", rateLimitMiddleware(apiLimiter), folderHandler.DeleteFolder)
		protected.PUT("/folders/:id", rateLimitMiddleware(apiLimiter), folderHandler.RenameFolder)
		protected.PUT("/folders/:id/move", rateLimitMiddleware(apiLimiter), folderHandler.MoveFolder)

		// Admin
		admin := protected.Group("/admin")
		{
			admin.GET("/users", rateLimitMiddleware(apiLimiter), adminHandler.GetAllUsers)
			admin.GET("/users/by-email/:email", rateLimitMiddleware(apiLimiter), adminHandler.GetUserByEmail)
			admin.POST("/users/:id/block", rateLimitMiddleware(apiLimiter), adminHandler.ToggleUserBlock)
			admin.POST("/users/:id/limit", rateLimitMiddleware(apiLimiter), adminHandler.SetUserStorageLimit)
			admin.POST("/users/clean-inactive", rateLimitMiddleware(apiLimiter), adminHandler.CleanInactiveUsers)
			admin.GET("/logs", rateLimitMiddleware(apiLimiter), adminHandler.GetAllLogs)
			admin.GET("/logs/user/:email", rateLimitMiddleware(apiLimiter), adminHandler.GetUserLogs)
			admin.POST("/settings/default-limit", rateLimitMiddleware(apiLimiter), adminHandler.SetDefaultLimit)
			admin.POST("/settings/max-file-size", rateLimitMiddleware(apiLimiter), adminHandler.SetMaxFileSize)
		}

		// User
		protected.GET("/user/me", rateLimitMiddleware(apiLimiter), func(c *gin.Context) {
			userID, exists := middleware.GetUserID(c)
			if !exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			user, err := userRepo.GetByID(userID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusOK, user)
		})
		protected.PUT("/user/me", rateLimitMiddleware(apiLimiter), userHandler.UpdateProfile)
		protected.POST("/user/avatar", rateLimitMiddleware(uploadLimiter), userHandler.UploadAvatar)

		// Library
		library := protected.Group("/library")
		{
			library.GET("/items", rateLimitMiddleware(apiLimiter), libraryHandler.GetItems)
			library.POST("/folders", rateLimitMiddleware(apiLimiter), libraryHandler.CreateFolder)
			library.POST("/upload", rateLimitMiddleware(uploadLimiter), libraryHandler.UploadFile)
			library.GET("/items/:id/download", rateLimitMiddleware(downloadLimiter), libraryHandler.DownloadFile)
			library.PUT("/items/:id", rateLimitMiddleware(apiLimiter), libraryHandler.UpdateItem)
			library.DELETE("/items/:id", rateLimitMiddleware(apiLimiter), libraryHandler.DeleteItem)
			library.GET("/admins", rateLimitMiddleware(apiLimiter), libraryHandler.ListAdmins)
			library.POST("/admins", rateLimitMiddleware(apiLimiter), libraryHandler.AddAdmin)
			library.DELETE("/admins/:email", rateLimitMiddleware(apiLimiter), libraryHandler.RemoveAdmin)
			library.GET("/stats", rateLimitMiddleware(apiLimiter), libraryHandler.GetLibraryStats)
		}

		// Schedule
		schedule := protected.Group("/schedule")
		{
			schedule.GET("/tasks", rateLimitMiddleware(apiLimiter), scheduleHandler.GetTasks)
			schedule.GET("/regions", rateLimitMiddleware(apiLimiter), scheduleHandler.GetAvailableRegions)
			schedule.GET("/tasks/by-region/:region", rateLimitMiddleware(apiLimiter), scheduleHandler.GetTasksByRegion)
			schedule.POST("/tasks", rateLimitMiddleware(apiLimiter), scheduleHandler.CreateTask)
			schedule.PUT("/tasks/:id", rateLimitMiddleware(apiLimiter), scheduleHandler.UpdateTask)
			schedule.PATCH("/tasks/:id/toggle", rateLimitMiddleware(apiLimiter), scheduleHandler.ToggleTaskComplete)
			schedule.DELETE("/tasks/:id", rateLimitMiddleware(apiLimiter), scheduleHandler.DeleteTask)
		}
	}

	return router
}

func rateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.getLimiter(ip).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func getAllowedOrigins() []string {
	if os.Getenv("ENV") == "production" {
		origins := os.Getenv("ALLOWED_ORIGINS")
		if origins == "" {
			return []string{"https://yourdomain.com"}
		}
		return strings.Split(origins, ",")
	}
	return []string{"*"}
}

func getEnv() string {
	env := os.Getenv("ENV")
	if env == "" {
		return "development"
	}
	return env
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
