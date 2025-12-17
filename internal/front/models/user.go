package models

import (
	"time"

)

type User struct {
	ID               string     `gorm:"primaryKey" json:"id"`
	UserTypeID       *uint      `gorm:"" json:"user_type_id"`
	FirstName        string     `gorm:"size:255;not null" json:"first_name"`
	LastName         string     `gorm:"size:255;not null" json:"last_name"`
	Email            string     `gorm:"size:191;not null;uniqueIndex" json:"email"`
	PhoneNumber      *string    `gorm:"size:191" json:"phone_number"`
	TypeUser         int        `gorm:"not null;default:0" json:"type_user"`
	WalletAddress    *string    `gorm:"type:text" json:"wallet_address"`
	Avatar           *string    `gorm:"size:100" json:"avatar"`
	FBID             *string    `gorm:"size:60" json:"fb_id"`
	ExpiredDate      *time.Time `gorm:"" json:"expired_date"`
	LastLogin        *time.Time `gorm:"" json:"last_login"`
	FirstLogin       *bool      `gorm:"" json:"first_login"`
	ActivationStatus bool       `gorm:"not null;default:false" json:"activation_status"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	IsBeta           bool       `gorm:"default:false" json:"is_beta"`
	IsBetaSent       *bool      `gorm:"" json:"is_beta_sent"`
	IsBetaATC        bool       `gorm:"default:false" json:"is_beta_atc_sent"`
	IsBetaETH        bool       `gorm:"default:false" json:"is_beta_eth_sent"`
	IsFree           bool       `gorm:"default:false" json:"is_free"`
	Step             int        `gorm:"default:0" json:"step"`
	Config           *string    `gorm:"type:json" json:"config"`
	OrganisationID   *uint      `gorm:"" json:"organisation_id"`
}

func (u User) TableName() string { return "users" }
