package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/luanlucolli/auth-catarinense/internal/database"
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

	username := strings.TrimSpace(request.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username é obrigatório"})
		return
	}

	user, err := h.store.GetUserByUsername(c.Request.Context(), username)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
			return
		}

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
	if err := h.store.UpdateSessionUUID(c.Request.Context(), user.ID, sessionUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao atualizar sessão"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		SessionUUID: sessionUUID,
		IsAdmin:     user.IsAdmin,
	})
}

func (h *Handler) Validate(c *gin.Context) {
	c.JSON(http.StatusOK, models.ValidateResponse{Valid: true})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var request models.CreateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}

	username := strings.TrimSpace(request.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username é obrigatório"})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao gerar hash da senha"})
		return
	}

	user, err := h.store.CreateUser(c.Request.Context(), database.CreateUserParams{
		Username:     username,
		PasswordHash: string(passwordHash),
		IsAdmin:      request.IsAdmin,
	})
	if err != nil {
		if errors.Is(err, database.ErrDuplicateUser) {
			c.JSON(http.StatusConflict, gin.H{"error": "username já cadastrado"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao criar usuário"})
		return
	}

	c.JSON(http.StatusCreated, models.CreateUserResponse{
		ID:       user.ID,
		Username: user.Username,
		IsAdmin:  user.IsAdmin,
		Active:   user.Active,
	})
}
