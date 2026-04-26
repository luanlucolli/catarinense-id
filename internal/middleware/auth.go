package middleware

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/luanlucolli/auth-catarinense/internal/database"
	"github.com/luanlucolli/auth-catarinense/internal/models"
)

const (
	ContextUserKey    = "authenticated_user"
	ContextAppKey     = "authenticated_app"
	ContextSessionKey = "authenticated_session"
)

type AuthMiddleware struct {
	store database.UserStore
}

func NewAuthMiddleware(store database.UserStore) *AuthMiddleware {
	return &AuthMiddleware{store: store}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionUUID, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token de autorização inválido"})
			return
		}

		appKey, err := extractAppKey(c.GetHeader("X-App-Key"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "app inválido ou inativo"})
			return
		}

		authContext, err := m.store.GetAuthContextBySessionAndAppKey(c.Request.Context(), sessionUUID, appKey)
		if err != nil {
			if errors.Is(err, database.ErrSessionNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
				return
			}

			log.Printf("erro ao validar sessão por app: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao validar sessão"})
			return
		}

		c.Set(ContextUserKey, authContext.User)
		c.Set(ContextAppKey, authContext.App)
		c.Set(ContextSessionKey, authContext.Session)
		c.Next()
	}
}

func (m *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(ContextUserKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "contexto de autenticação ausente"})
			return
		}

		user, ok := value.(models.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "contexto de autenticação inválido"})
			return
		}

		if !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "acesso restrito a administradores"})
			return
		}

		c.Next()
	}
}

func extractBearerToken(headerValue string) (string, error) {
	parts := strings.Fields(headerValue)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty token")
	}

	if _, err := uuid.Parse(token); err != nil {
		return "", err
	}

	return token, nil
}

func extractAppKey(headerValue string) (string, error) {
	appKey := strings.TrimSpace(headerValue)
	if appKey == "" {
		return "", errors.New("empty app key")
	}

	return appKey, nil
}
