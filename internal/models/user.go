package models

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// RoleType определяет тип роли пользователя
type RoleType string

const (
	RoleAdmin  RoleType = "admin"
	RoleUser   RoleType = "user"
	RoleVassal RoleType = "vassal"
)

// User определяет модель пользователя в системе
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	TelegramID   int64     `json:"telegram_id" db:"telegram_id"`
	Role         RoleType  `json:"role" db:"role"`
	Certificate  string    `json:"-" db:"certificate"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	LastLoginAt  time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	InvitedBy    int64     `json:"invited_by,omitempty" db:"invited_by"`
	TrafficLimit int64     `json:"traffic_limit,omitempty" db:"traffic_limit"`
}

// GetRoleLimits возвращает ограничения на основе роли пользователя
func (u *User) GetRoleLimits() RoleLimits {
	switch u.Role {
	case RoleAdmin:
		return RoleLimits{
			MaxInvites:             -1, // безлимитно
			MaxVPNConnections:      -1, // безлимитно
			CanAddRoutes:           true,
			CanViewLogs:            true,
			CanManageUsers:         true,
			CanManageInvites:       true,
			CanViewInviteTree:      true,
			CanOverrideAdminRoutes: true,
		}
	case RoleUser:
		return RoleLimits{
			MaxInvites:             4,
			MaxVPNConnections:      1,
			CanAddRoutes:           true,
			CanViewLogs:            false,
			CanManageUsers:         false,
			CanManageInvites:       true,
			CanViewInviteTree:      true,
			CanOverrideAdminRoutes: false,
		}
	case RoleVassal:
		return RoleLimits{
			MaxInvites:             0,
			MaxVPNConnections:      1,
			CanAddRoutes:           false,
			CanViewLogs:            false,
			CanManageUsers:         false,
			CanManageInvites:       false,
			CanViewInviteTree:      false,
			CanOverrideAdminRoutes: false,
		}
	default:
		return RoleLimits{}
	}
}

// RoleLimits определяет ограничения для роли
type RoleLimits struct {
	MaxInvites             int  // Максимальное количество инвайтов
	MaxVPNConnections      int  // Максимальное количество подключений VPN
	CanAddRoutes           bool // Может ли добавлять маршруты
	CanViewLogs            bool // Может ли просматривать логи
	CanManageUsers         bool // Может ли управлять пользователями
	CanManageInvites       bool // Может ли управлять инвайтами
	CanViewInviteTree      bool // Может ли видеть дерево инвайтов
	CanOverrideAdminRoutes bool // Может ли изменять админские запреты
}

// ParseCertificate возвращает x509 сертификат из хранимого PEM формата
func (u *User) ParseCertificate() (*x509.Certificate, error) {
	if u.Certificate == "" {
		return nil, errors.New("certificate is empty")
	}

	block, _ := pem.Decode([]byte(u.Certificate))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

// UserTraffic представляет статистику трафика пользователя
type UserTraffic struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Bytes     int64     `json:"bytes" db:"bytes"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
}
