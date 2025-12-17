package services

import (
	"gorm.io/gorm"
)

// AdminServices groups all admin services together. The specific service
// interfaces are defined in their own files (product.go, user.go, customer.go).
type AdminServices struct {
	DB   *gorm.DB
	Auth *AuthService
}

// Default is the package-level services instance used by handlers.
var Default *AdminServices

// SetDefault sets the package-level services instance.
func SetDefault(s *AdminServices) {
	Default = s
}

// GetDefault returns the current package-level services instance.
func GetDefault() *AdminServices {
	return Default
}
