package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	// Rate limiters
	uploadLimiter := rate.NewLimiter(5, 10)
	authLimiter := rate.NewLimiter(3, 5)
	apiLimiter := rate.NewLimiter(10, 30)

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	if err := database.CreateTables(db); err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	log.Println("Database connected successfully!")
	avatarBasePath := filepath.Join(cfg.UploadPath, "avatars")
	if err := os.MkdirAll(avatarBasePath, 0755); err != nil {
		log.Printf("Warning: could not create avatars base dir: %v", err)
	}

	// Репозитории
	userRepo := repository.NewUserRepository(db)
	fileRepo := repository.NewFileRepository(db)
	oauthRepo := repository.NewOAuthRepository(db)
	libRepo := repository.NewLibraryRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	auditService := services.NewAuditService(auditRepo)
	scheduleRepo := repository.NewScheduleRepository(db)

	// Утилиты
	jwtUtil := utils.NewJWTUtil(cfg.JWTSecret, cfg.JWTExpiresIn)
	encryptionService := crypto.NewEncryptionService(cfg.JWTSecret)

	// Сервисы
	storageService := services.NewStorageService(cfg.UploadPath)
	emailService := services.NewEmailService()
	libraryService := services.NewLibraryService(libRepo, userRepo, encryptionService, cfg.UploadPath, auditService)

	authService := services.NewAuthService(userRepo, oauthRepo, jwtUtil, emailService)
	fileService := services.NewFileService(fileRepo, storageService, auditService, userRepo)

	// Инициализация администраторов
	if len(cfg.LibraryAdmins) > 0 {
		var initiatorID uuid.UUID
		err := db.QueryRow("SELECT id FROM users WHERE email = $1", cfg.LibraryAdmins[0]).Scan(&initiatorID)
		if err != nil {
			initiatorID = uuid.New()
			log.Printf("Warning: admin user %s not found, using temporary ID", cfg.LibraryAdmins[0])
		}
		if err := libraryService.InitAdminsFromConfig(cfg.LibraryAdmins, initiatorID); err != nil {
			log.Printf("Warning: failed to init admins: %v", err)
		} else {
			log.Printf("✅ Initialized %d admins from .env", len(cfg.LibraryAdmins))
		}
	}

	// OAuth провайдеры
	googleProvider := oauth.NewGoogleProvider(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("GOOGLE_REDIRECT_URL"),
	)
	yandexProvider := oauth.NewYandexProvider(
		os.Getenv("YANDEX_CLIENT_ID"),
		os.Getenv("YANDEX_CLIENT_SECRET"),
		os.Getenv("YANDEX_REDIRECT_URL"),
	)
	vkProvider := oauth.NewVKProvider(
		os.Getenv("VK_CLIENT_ID"),
		os.Getenv("VK_CLIENT_SECRET"),
		os.Getenv("VK_REDIRECT_URL"),
	)

	// Сессии
	sessionConfig := &middleware.SessionConfig{
		SecretKey: os.Getenv("SESSION_SECRET"),
		MaxAge:    86400 * 7,
		Secure:    false,
		HttpOnly:  true,
		SameSite:  http.SameSiteLaxMode,
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

	// Роутер
	router := gin.Default()

	// CORS - ПРОСТАЯ ВЕРСИЯ ДЛЯ РАЗРАБОТКИ
	router.Use(cors.Default())

	// Только базовые middleware
	router.Use(sessionManager.SessionMiddleware())

	// ========== СТАТИЧЕСКИЕ ФАЙЛЫ ==========
	router.Static("/css", "./web/css")
	router.Static("/js", "./web/js")
	router.Static("/uploads", cfg.UploadPath)

	// HTML страницы
	router.StaticFile("/", "./web/index.html")
	router.StaticFile("/login", "./web/login.html")
	router.StaticFile("/register", "./web/register.html")
	router.StaticFile("/admin", "./web/admin.html")
	router.StaticFile("/admin.html", "./web/admin.html")
	router.StaticFile("/dashboard", "./web/dashboard.html")
	router.StaticFile("/library", "./web/library.html")
	router.StaticFile("/verify", "./web/verify.html")
	router.StaticFile("/status", "./web/status.html")

	// Редиректы
	router.GET("/login.html", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/login") })
	router.GET("/register.html", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/register") })
	router.GET("/dashboard.html", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/dashboard") })
	router.GET("/library.html", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/library") })

	// API маршруты
	auth := router.Group("/api/auth")
	{
		auth.POST("/register", rateLimitMiddleware(authLimiter), authHandler.Register)
		auth.POST("/login", rateLimitMiddleware(authLimiter), authHandler.Login)
		auth.POST("/verify", rateLimitMiddleware(authLimiter), authHandler.VerifyEmail)
		auth.POST("/resend-code", rateLimitMiddleware(authLimiter), authHandler.ResendCode)
		auth.GET("/:provider/login", oauthHandler.Login)
		auth.GET("/:provider/callback", oauthHandler.Callback)
	}

	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware(jwtUtil))
	{
		protected.POST("/files/upload", rateLimitMiddleware(uploadLimiter), fileHandler.UploadFile)
		protected.GET("/files", rateLimitMiddleware(apiLimiter), fileHandler.ListFiles)
		protected.GET("/storage/stats", rateLimitMiddleware(apiLimiter), fileHandler.GetStorageStats)
		protected.GET("/files/:id", rateLimitMiddleware(apiLimiter), fileHandler.GetFile)
		protected.GET("/files/:id/download", rateLimitMiddleware(apiLimiter), fileHandler.DownloadFile)
		protected.DELETE("/files/:id", rateLimitMiddleware(apiLimiter), fileHandler.DeleteFile)
		protected.PUT("/files/:id", rateLimitMiddleware(apiLimiter), fileHandler.RenameFile)

		protected.POST("/folders", rateLimitMiddleware(apiLimiter), folderHandler.CreateFolder)
		protected.GET("/folders/:id/contents", rateLimitMiddleware(apiLimiter), folderHandler.GetFolderContents)
		protected.DELETE("/folders/:id", rateLimitMiddleware(apiLimiter), folderHandler.DeleteFolder)
		protected.PUT("/folders/:id", rateLimitMiddleware(apiLimiter), folderHandler.RenameFolder)
		protected.PUT("/files/:id/move", rateLimitMiddleware(apiLimiter), fileHandler.MoveFile)
		protected.PUT("/folders/:id/move", rateLimitMiddleware(apiLimiter), folderHandler.MoveFolder)

		// Админ маршруты
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

		// Библиотека
		library := protected.Group("/library")
		{
			library.GET("/items", rateLimitMiddleware(apiLimiter), libraryHandler.GetItems)
			library.POST("/folders", rateLimitMiddleware(apiLimiter), libraryHandler.CreateFolder)
			library.POST("/upload", rateLimitMiddleware(uploadLimiter), libraryHandler.UploadFile)
			library.GET("/items/:id/download", rateLimitMiddleware(apiLimiter), libraryHandler.DownloadFile)
			library.PUT("/items/:id", rateLimitMiddleware(apiLimiter), libraryHandler.UpdateItem)
			library.DELETE("/items/:id", rateLimitMiddleware(apiLimiter), libraryHandler.DeleteItem)
			library.GET("/admins", rateLimitMiddleware(apiLimiter), libraryHandler.ListAdmins)
			library.POST("/admins", rateLimitMiddleware(apiLimiter), libraryHandler.AddAdmin)
			library.DELETE("/admins/:email", rateLimitMiddleware(apiLimiter), libraryHandler.RemoveAdmin)
			library.GET("/stats", rateLimitMiddleware(apiLimiter), libraryHandler.GetLibraryStats)
		}

		// ========== РАСПИСАНИЕ МЕРОПРИЯТИЙ ==========
		// ========== РАСПИСАНИЕ ЗАДАЧ ==========
		schedule := protected.Group("/schedule")
		{
			schedule.GET("/tasks", rateLimitMiddleware(apiLimiter), scheduleHandler.GetTasks)
			schedule.POST("/tasks", rateLimitMiddleware(apiLimiter), scheduleHandler.CreateTask)
			schedule.PUT("/tasks/:id", rateLimitMiddleware(apiLimiter), scheduleHandler.UpdateTask)
			schedule.PATCH("/tasks/:id/toggle", rateLimitMiddleware(apiLimiter), scheduleHandler.ToggleTaskComplete)
			schedule.DELETE("/tasks/:id", rateLimitMiddleware(apiLimiter), scheduleHandler.DeleteTask)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.ServerPort
	}

	log.Printf("Server starting on port %s (ENV: %s)", port, os.Getenv("ENV"))
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func rateLimitMiddleware(limiter *rate.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			c.Abort()
			return
		}
		c.Next()
	}
}
