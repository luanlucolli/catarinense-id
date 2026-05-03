package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/luanlucolli/auth-catarinense/internal/database"
	"github.com/luanlucolli/auth-catarinense/internal/middleware"
	"github.com/luanlucolli/auth-catarinense/internal/models"
)

type Handler struct {
	store database.UserStore
}

func NewHandler(store database.UserStore) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Login(c *gin.Context) {
	var request models.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}

	appKey := strings.TrimSpace(c.GetHeader("X-App-Key"))
	if appKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "app inválido ou inativo"})
		return
	}

	app, err := h.store.GetActiveAppByAPIKey(c.Request.Context(), appKey)
	if err != nil {
		if errors.Is(err, database.ErrAppNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "app inválido ou inativo"})
			return
		}

		log.Printf("erro ao buscar app ativo por api key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao validar app"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email é obrigatório"})
		return
	}

	user, err := h.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
			return
		}

		log.Printf("erro ao buscar usuário por email %q: %v", email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao buscar usuário"})
		return
	}

	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{"error": "usuário inativo"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return
	}

	sessionUUID := uuid.NewString()
	session, err := h.store.UpsertSession(c.Request.Context(), database.UpsertSessionParams{
		UserID:      user.ID,
		AppID:       app.ID,
		SessionUUID: sessionUUID,
	})
	if err != nil {
		log.Printf("erro ao criar sessão para user_id=%d app_id=%d: %v", user.ID, app.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao atualizar sessão"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		SessionUUID: session.SessionUUID,
		IsAdmin:     user.IsAdmin,
	})
}

func (h *Handler) Validate(c *gin.Context) {
	c.JSON(http.StatusOK, models.ValidateResponse{Valid: true})
}

func (h *Handler) Me(c *gin.Context) {
	value, exists := c.Get(middleware.ContextUserKey)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "usuário autenticado ausente no contexto"})
		return
	}

	user, ok := value.(models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao interpretar usuário autenticado do contexto"})
		return
	}

	response := models.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		IsAdmin:   user.IsAdmin,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) Logout(c *gin.Context) {
	value, exists := c.Get(middleware.ContextSessionKey)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "contexto de autenticação ausente"})
		return
	}

	session, ok := value.(models.Session)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "contexto de autenticação inválido"})
		return
	}

	if err := h.store.DeleteSessionByID(c.Request.Context(), session.ID); err != nil {
		if errors.Is(err, database.ErrSessionNotFound) {
			c.JSON(http.StatusOK, gin.H{"message": "logout realizado com sucesso"})
			return
		}

		log.Printf("erro ao remover sessão id=%d: %v", session.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao finalizar sessão"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logout realizado com sucesso"})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var request models.CreateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email é obrigatório"})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("erro ao gerar hash da senha para email=%q: %v", email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao gerar hash da senha"})
		return
	}

	user, err := h.store.CreateUser(c.Request.Context(), database.CreateUserParams{
		Email:        email,
		PasswordHash: string(passwordHash),
		IsAdmin:      request.IsAdmin,
	})
	if err != nil {
		if errors.Is(err, database.ErrDuplicateUser) {
			c.JSON(http.StatusConflict, gin.H{"error": "email já cadastrado"})
			return
		}

		log.Printf("erro ao criar usuário %q: %v", email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao criar usuário"})
		return
	}

	c.JSON(http.StatusCreated, models.CreateUserResponse{
		ID:      user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
		Active:  user.Active,
	})
}
