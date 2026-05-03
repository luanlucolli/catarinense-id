package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/luanlucolli/auth-catarinense/internal/database"
	"github.com/luanlucolli/auth-catarinense/internal/handlers"
	"github.com/luanlucolli/auth-catarinense/internal/middleware"
)

func main() {
	loadEnv()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL não definida")
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.ReleaseMode
	}
	gin.SetMode(ginMode)
	log.Printf("GIN mode: %s", gin.Mode())

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	repo, err := openRepositoryWithRetry(ctx, databaseURL)
	if err != nil {
		log.Fatalf("falha ao inicializar banco: %v", err)
	}
	defer repo.Close()

	handler := handlers.NewHandler(repo)
	authMiddleware := middleware.NewAuthMiddleware(repo)
	globalRateLimit := loadRateLimitConfig(
		"RATE_LIMIT_REQUESTS",
		"RATE_LIMIT_WINDOW",
		"RATE_LIMIT_BURST",
		middleware.RateLimitConfig{
			Requests: 120,
			Window:   time.Minute,
			Burst:    20,
		},
	)
	loginRateLimit := loadRateLimitConfig(
		"LOGIN_RATE_LIMIT_REQUESTS",
		"LOGIN_RATE_LIMIT_WINDOW",
		"LOGIN_RATE_LIMIT_BURST",
		middleware.RateLimitConfig{
			Requests: 10,
			Window:   time.Minute,
			Burst:    5,
		},
	)

	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Fatalf("falha ao configurar trusted proxies: %v", err)
	}
	router.Use(middleware.NewRateLimiter(globalRateLimit))
	registerRoutes(router, handler, authMiddleware, middleware.NewRateLimiter(loginRateLimit))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("API pronta para receber conexões em :%s", port)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("falha ao iniciar servidor: %v", err)
	}
}

func loadEnv() {
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		return
	}

	envPath, err := findEnvFile()
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			log.Println("aviso: arquivo .env não encontrado; usando variáveis do sistema")
			return
		}

		log.Printf("aviso: falha ao localizar .env: %v", err)
		return
	}

	if err := godotenv.Load(envPath); err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			log.Println("aviso: arquivo .env não encontrado; usando variáveis do sistema")
			return
		}

		log.Printf("aviso: falha ao carregar .env: %v", err)
	}
}

func findEnvFile() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	currentDir := workingDir
	for {
		candidate := filepath.Join(currentDir, ".env")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", &os.PathError{Op: "stat", Path: ".env", Err: os.ErrNotExist}
		}

		currentDir = parentDir
	}
}

func registerRoutes(router *gin.Engine, handler *handlers.Handler, authMiddleware *middleware.AuthMiddleware, loginRateLimiter gin.HandlerFunc) {
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Auth Catarinense online"})
	})

	router.POST("/login", loginRateLimiter, handler.Login)

	protected := router.Group("/")
	protected.Use(authMiddleware.RequireAuth())
	protected.GET("/validate", handler.Validate)
	protected.GET("/me", handler.Me)
	protected.POST("/logout", handler.Logout)

	admin := router.Group("/admin")
	admin.Use(authMiddleware.RequireAuth(), authMiddleware.RequireAdmin())
	admin.POST("/users", handler.CreateUser)
}

func openRepositoryWithRetry(ctx context.Context, databaseURL string) (*database.Repository, error) {
	const (
		maxAttempts    = 4
		baseBackoff    = time.Second
		perAttemptTime = 12 * time.Second
	)

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return nil, fmt.Errorf("timeout ao inicializar banco após %d tentativa(s): %w", attempt-1, lastErr)
			}

			return nil, fmt.Errorf("timeout ao inicializar banco: %w", err)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, min(perAttemptTime, time.Until(contextDeadlineOrNow(ctx))))
		repo, err := database.NewRepository(attemptCtx, databaseURL)
		cancel()
		if err == nil {
			return repo, nil
		}

		lastErr = err
		if attempt == maxAttempts {
			break
		}

		backoff := time.Duration(attempt) * baseBackoff
		log.Printf("aviso: tentativa %d/%d de inicializar banco falhou: %v", attempt, maxAttempts, err)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("timeout ao inicializar banco após %d tentativa(s): %w", attempt, lastErr)
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf("falha ao inicializar banco após %d tentativa(s): %w", maxAttempts, lastErr)
}

func contextDeadlineOrNow(ctx context.Context) time.Time {
	deadline, ok := ctx.Deadline()
	if !ok {
		return time.Now().Add(12 * time.Second)
	}

	return deadline
}

func loadRateLimitConfig(
	requestsEnv string,
	windowEnv string,
	burstEnv string,
	defaults middleware.RateLimitConfig,
) middleware.RateLimitConfig {
	return middleware.RateLimitConfig{
		Requests: parsePositiveIntEnv(requestsEnv, defaults.Requests),
		Window:   parsePositiveDurationEnv(windowEnv, defaults.Window),
		Burst:    parsePositiveIntEnv(burstEnv, defaults.Burst),
	}
}

func parsePositiveIntEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Printf("aviso: %s inválida (%q); usando %d", name, value, fallback)
		return fallback
	}

	return parsed
}

func parsePositiveDurationEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		log.Printf("aviso: %s inválida (%q); usando %s", name, value, fallback)
		return fallback
	}

	return parsed
}
