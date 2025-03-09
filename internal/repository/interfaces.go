package repository

import (
	"context"

	"github.com/yourusername/eidolon/internal/models"
)

// UserRepository определяет интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id int64) (*models.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, offset, limit int) ([]*models.User, error)
	CountByInviter(ctx context.Context, inviterID int64) (int, error)
	GetInvitedUsers(ctx context.Context, inviterID int64) ([]*models.User, error)
}

// InviteRepository определяет интерфейс для работы с инвайт-кодами
type InviteRepository interface {
	Create(ctx context.Context, invite *models.InviteCode) error
	GetByCode(ctx context.Context, code string) (*models.InviteCode, error)
	GetByID(ctx context.Context, id int64) (*models.InviteCode, error)
	Update(ctx context.Context, invite *models.InviteCode) error
	Delete(ctx context.Context, id int64) error
	ListByCreator(ctx context.Context, creatorID int64) ([]*models.InviteCode, error)
	CountActiveByCreator(ctx context.Context, creatorID int64) (int, error)
}

// RouteRepository определяет интерфейс для работы с маршрутами
type RouteRepository interface {
	Create(ctx context.Context, route *models.Route) error
	GetByID(ctx context.Context, id int64) (*models.Route, error)
	Update(ctx context.Context, route *models.Route) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, routeType models.RouteType) ([]*models.Route, error)

	// ASN маршруты
	CreateASN(ctx context.Context, route *models.ASNRoute) error
	GetASNByID(ctx context.Context, id int64) (*models.ASNRoute, error)
	ListASN(ctx context.Context, routeType models.RouteType) ([]*models.ASNRoute, error)

	// Группы маршрутов
	CreateGroup(ctx context.Context, group *models.RouteGroup) error
	GetGroupByID(ctx context.Context, id int64) (*models.RouteGroup, error)
	AddRouteToGroup(ctx context.Context, groupID, routeID int64) error
	RemoveRouteFromGroup(ctx context.Context, groupID, routeID int64) error
	GetRoutesInGroup(ctx context.Context, groupID int64) ([]*models.Route, error)

	// Связь с пользователями
	AssignRouteToUser(ctx context.Context, userRoute *models.UserRoute) error
	GetUserRoutes(ctx context.Context, userID int64) ([]*models.Route, error)
	AssignGroupToUser(ctx context.Context, userGroup *models.UserRouteGroup) error
	GetUserGroups(ctx context.Context, userID int64) ([]*models.RouteGroup, error)
	UnassignRouteFromUser(ctx context.Context, userID, routeID int64) error
	UnassignGroupFromUser(ctx context.Context, userID, groupID int64) error
}

// TrafficRepository определяет интерфейс для работы с данными о трафике
type TrafficRepository interface {
	LogTraffic(ctx context.Context, traffic *models.UserTraffic) error
	GetUserTraffic(ctx context.Context, userID int64, from, to int64) ([]*models.UserTraffic, error)
	GetTotalUserTraffic(ctx context.Context, userID int64) (int64, error)
}

// Repository объединяет все репозитории
type Repository interface {
	User() UserRepository
	Invite() InviteRepository
	Route() RouteRepository
	Traffic() TrafficRepository
}
