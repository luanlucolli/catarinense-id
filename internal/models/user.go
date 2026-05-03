package models

import "time"

type User struct {
	ID           int32     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

type App struct {
	ID        int32     `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"-"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	ID          int32     `json:"id"`
	UserID      int32     `json:"user_id"`
	AppID       int32     `json:"app_id"`
	SessionUUID string    `json:"session_uuid"`
	LastActive  time.Time `json:"last_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type AuthContext struct {
	User    User
	App     App
	Session Session
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	SessionUUID string `json:"session_uuid"`
	IsAdmin     bool   `json:"is_admin"`
}

type CreateUserRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	IsAdmin  bool   `json:"is_admin"`
}

type CreateUserResponse struct {
	ID      int32  `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
	Active  bool   `json:"active"`
}

type UserResponse struct {
	ID        int32     `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
