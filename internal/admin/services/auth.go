package services

import (
	"go_framework/internal/admin/models"
	authjwt "go_framework/internal/auth"

	"gorm.io/gorm"
)

// AuthService is a concrete GORM-backed authentication service.
type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

func (g *AuthService) Me(accessToken string) (*models.Admin, error) {
	sub, err := authjwt.ParseAccessToken(accessToken)
	if err != nil {
		return nil, err
	}
	var u models.Admin
	if err := g.db.First(&u, "id = ?", sub).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
