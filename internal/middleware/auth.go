package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/luanlucolli/auth-catarinense/internal/database"
	"github.com/luanlucolli/auth-catarinense/internal/models"
)

const ContextUserKey = "authenticated_user"

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

		user, err := m.store.GetActiveUserBySessionUUID(c.Request.Context(), sessionUUID)
		if err != nil {
			if errors.Is(err, database.ErrSessionNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
				return
			}

			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao validar sessão"})
			return
		}

		c.Set(ContextUserKey, user)
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
