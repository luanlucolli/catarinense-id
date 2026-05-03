package models

import "time"

type User struct {
	ID           int32     `json:"id"`
	Username     string    `json:"username"`
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
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	SessionUUID string `json:"session_uuid"`
	IsAdmin     bool   `json:"is_admin"`
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	IsAdmin  bool   `json:"is_admin"`
}

type CreateUserResponse struct {
	ID       int32  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	Active   bool   `json:"active"`
}

type ValidateResponse struct {
	Valid bool `json:"valid"`
}

type UserResponse struct {
	ID        int32     `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
