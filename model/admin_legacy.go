//go:build tt
// +build tt

// Package model — admin_legacy.go
//
// 独立管理员表（Admin）及其 CRUD 函数。
// 当前架构中 admin 身份通过 upstream users 表 (role>=10) + UserExtension.AdminRole 实现，
// 由 AdminAuthBridge 中间件桥接，不依赖此表。
// 保留此文件用于未来可能的独立 admin 登录场景。
package model

import (
	"time"
)

// Admin 独立管理员表（当前未用于身份验证，见文件顶部说明）
type Admin struct {
	Id           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Email        string    `json:"email" gorm:"uniqueIndex;size:128;not null"`
	PasswordHash string    `json:"-" gorm:"size:128;not null"`
	Role         AdminRole `json:"role" gorm:"size:20;default:'operator'"`
	TOTPSecret   string    `json:"-" gorm:"size:64"`
	IsActive     bool      `json:"is_active" gorm:"default:true"`

	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

func (Admin) TableName() string {
	return "admins"
}

func GetAdminById(id uint) (*Admin, error) {
	var admin Admin
	err := DB.First(&admin, id).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func GetAdminByUsername(username string) (*Admin, error) {
	var admin Admin
	err := DB.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func CreateAdmin(username, email, passwordHash string, role AdminRole) (*Admin, error) {
	admin := Admin{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		IsActive:     true,
	}
	err := DB.Create(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func UpdateAdminTOTP(adminId uint, totpSecret string) error {
	return DB.Model(&Admin{}).Where("id = ?", adminId).
		Update("totp_secret", totpSecret).Error
}

func UpdateAdminLastLogin(adminId uint) error {
	now := time.Now()
	return DB.Model(&Admin{}).Where("id = ?", adminId).
		Update("last_login_at", now).Error
}
