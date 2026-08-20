package model

import "time"

type Role string

const (
	RoleViewer  Role = "viewer"
	RoleCreator Role = "creator"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash *string   `json:"-"`
	DisplayName  string    `json:"display_name"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
