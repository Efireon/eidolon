package models

import (
	"net"
	"time"
)

// RouteType определяет тип маршрута
type RouteType string

const (
	RouteTypeDefault RouteType = "default" // Маршрут по умолчанию
	RouteTypeCustom  RouteType = "custom"  // Пользовательский маршрут
	RouteTypeASN     RouteType = "asn"     // Маршрут по ASN
	RouteTypeBlock   RouteType = "blocked" // Заблокированный маршрут
)

// Route определяет маршрут для VPN
type Route struct {
	ID          int64     `json:"id" db:"id"`
	Network     string    `json:"network" db:"network"` // CIDR нотация
	Description string    `json:"description" db:"description"`
	Type        RouteType `json:"type" db:"type"`
	CreatedBy   int64     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ParseNetwork преобразует строку CIDR в объект IPNet
func (r *Route) ParseNetwork() (*net.IPNet, error) {
	_, ipNet, err := net.ParseCIDR(r.Network)
	if err != nil {
		return nil, err
	}
	return ipNet, nil
}

// ASNRoute определяет маршрут на основе ASN
type ASNRoute struct {
	ID          int64     `json:"id" db:"id"`
	ASN         int       `json:"asn" db:"asn"`
	Description string    `json:"description" db:"description"`
	CreatedBy   int64     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	Type        RouteType `json:"type" db:"type"`
}

// UserRoute связывает маршрут с пользователем
type UserRoute struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	RouteID   int64     `json:"route_id" db:"route_id"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RouteGroup группирует маршруты для удобства управления
type RouteGroup struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedBy   int64     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// RouteGroupItem связывает маршрут с группой
type RouteGroupItem struct {
	GroupID int64 `json:"group_id" db:"group_id"`
	RouteID int64 `json:"route_id" db:"route_id"`
}

// UserRouteGroup связывает группу маршрутов с пользователем
type UserRouteGroup struct {
	UserID    int64     `json:"user_id" db:"user_id"`
	GroupID   int64     `json:"group_id" db:"group_id"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
