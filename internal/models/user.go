package models

import "time"

type User struct {
	ID           int32     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	Active       bool      `json:"active"`
	SessionUUID  *string   `json:"session_uuid,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
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
