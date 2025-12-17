package services

import (
	admin "go_framework/internal/admin/services"
)

// StoreServices exposes a curated subset of admin services that are safe to use in public storefront handlers.
type StoreServices struct{}

var (
	defaultStoreServices *StoreServices
)

// NewStoreServices builds store-focused services on top of admin services.
func NewStoreServices(adminSvc *admin.AdminServices) *StoreServices {
	if adminSvc == nil {
		return nil
	}
	return &StoreServices{}
}

// SetDefault stores the provided StoreServices for reuse by handlers.
func SetDefault(svc *StoreServices) {
	defaultStoreServices = svc
}

// GetDefault returns the shared StoreServices instance.
func GetDefault() *StoreServices {
	return defaultStoreServices
}
