package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
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
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo, err := database.NewRepository(ctx, databaseURL)
	if err != nil {
		log.Fatalf("falha ao inicializar banco: %v", err)
	}
	defer repo.Close()

	handler := handlers.NewHandler(repo)
	authMiddleware := middleware.NewAuthMiddleware(repo)

	router := gin.Default()
	registerRoutes(router, handler, authMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("API pronta para receber conexões em :%s", port)

	if err := router.Run(":" + port); err != nil {
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

func registerRoutes(router *gin.Engine, handler *handlers.Handler, authMiddleware *middleware.AuthMiddleware) {
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Auth Catarinense online"})
	})

	router.POST("/login", handler.Login)

	protected := router.Group("/")
	protected.Use(authMiddleware.RequireAuth())
	protected.GET("/validate", handler.Validate)

	admin := router.Group("/admin")
	admin.Use(authMiddleware.RequireAuth(), authMiddleware.RequireAdmin())
	admin.POST("/users", handler.CreateUser)
}
