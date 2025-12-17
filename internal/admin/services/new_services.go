package services

import "gorm.io/gorm"

// NewServices constructs AdminServices backed by a *gorm.DB.
func NewServices(db *gorm.DB) *AdminServices {
	return &AdminServices{
		DB:   db,
		Auth: NewAuthService(db),
	}
}
