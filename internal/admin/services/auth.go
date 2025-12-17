package services

import (
	"errors"
	"time"

	"go_framework/internal/admin/models"
	authjwt "go_framework/internal/auth"
	"go_framework/internal/uuid"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService is a concrete GORM-backed authentication service.
type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

func (g *AuthService) Login(email, password string) (*models.Admin, string, string, error) {
	var u models.Admin
	if err := g.db.Where("email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", errors.New("invalid credentials")
		}
		return nil, "", "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, "", "", errors.New("invalid credentials")
	}
	access, err := authjwt.SignAccessToken(u.ID)
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := authjwt.SignRefreshToken(u.ID)
	if err != nil {
		return nil, "", "", err
	}
	return &u, access, refresh, nil
}

func (g *AuthService) Register(u *models.Admin, password string) (*models.Admin, string, string, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, "", "", err
	}
	now := time.Now().UTC()
	u.ID = id
	u.CreatedAt = now
	u.UpdatedAt = now
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", "", err
	}
	u.Password = string(hashed)
	if err := g.db.Create(u).Error; err != nil {
		return nil, "", "", err
	}
	access, err := authjwt.SignAccessToken(u.ID)
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := authjwt.SignRefreshToken(u.ID)
	if err != nil {
		return nil, "", "", err
	}
	return u, access, refresh, nil
}

func (g *AuthService) ForgotPassword(email string) error {
	// placeholder
	return nil
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
