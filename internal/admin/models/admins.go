package models

import (
	"time"

	"gorm.io/gorm"
)

type Admin struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	FirstName     string    `gorm:"size:191;not null" json:"first_name"`
	LastName      string    `gorm:"size:191;not null" json:"last_name"`
	Email         string    `gorm:"size:191;not null;unique" json:"email"`
	Password      string    `gorm:"size:191;not null" json:"password"`
	RememberToken string    `gorm:"size:100" json:"remember_token"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Privilege     string    `gorm:"size:191;default:admin" json:"privilege"`
	gorm.Model
}

func (a Admin) TableName() string { return "admins" }
