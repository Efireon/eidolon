package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"eidolon/internal/models"
	"eidolon/internal/repository"

	"github.com/sirupsen/logrus"
)

// RouteService предоставляет методы для управления маршрутами
type RouteService struct {
	repo   repository.Repository
	logger *logrus.Logger
}

// NewRouteService создает новый сервис управления маршрутами
func NewRouteService(repo repository.Repository, logger *logrus.Logger) *RouteService {
	return &RouteService{
		repo:   repo,
		logger: logger,
	}
}

// CreateRoute создает новый маршрут
func (s *RouteService) CreateRoute(ctx context.Context, route *models.Route) error {
	// Проверяем формат CIDR
	_, ipNet, err := net.ParseCIDR(route.Network)
	if err != nil {
		return fmt.Errorf("invalid CIDR format for Network: %w", err)
	}

	// Устанавливаем нормализованное значение CIDR
	route.Network = ipNet.String()

	// Если тип не указан, устанавливаем по умолчанию
	if route.Type == "" {
		route.Type = models.RouteTypeCustom
	}

	// Устанавливаем время создания, если не указано
	if route.CreatedAt.IsZero() {
		route.CreatedAt = time.Now()
	}

	// Создаем маршрут в базе данных
	if err := s.repo.Route().Create(ctx, route); err != nil {
		return fmt.Errorf("failed to create route in database: %w", err)
	}

	return nil
}

// GetRouteByID получает маршрут по ID
func (s *RouteService) GetRouteByID(ctx context.Context, id int64) (*models.Route, error) {
	route, err := s.repo.Route().GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get route by ID: %w", err)
	}
	return route, nil
}

// UpdateRoute обновляет существующий маршрут
func (s *RouteService) UpdateRoute(ctx context.Context, route *models.Route) error {
	// Проверяем формат CIDR, если сеть изменилась
	if route.Network != "" {
		_, ipNet, err := net.ParseCIDR(route.Network)
		if err != nil {
			return fmt.Errorf("invalid CIDR format for Network: %w", err)
		}
		route.Network = ipNet.String()
	}

	// Обновляем маршрут в базе данных
	if err := s.repo.Route().Update(ctx, route); err != nil {
		return fmt.Errorf("failed to update route in database: %w", err)
	}

	return nil
}

// DeleteRoute удаляет маршрут
func (s *RouteService) DeleteRoute(ctx context.Context, id int64) error {
	// Удаляем маршрут из базы данных
	if err := s.repo.Route().Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete route from database: %w", err)
	}
	return nil
}

// ListRoutes возвращает список маршрутов по типу
func (s *RouteService) ListRoutes(ctx context.Context, routeType models.RouteType) ([]*models.Route, error) {
	routes, err := s.repo.Route().List(ctx, routeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes list: %w", err)
	}
	return routes, nil
}

// CreateASNRoute создает новый ASN маршрут
func (s *RouteService) CreateASNRoute(ctx context.Context, route *models.ASNRoute) error {
	// Если тип не указан, устанавливаем по умолчанию
	if route.Type == "" {
		route.Type = models.RouteTypeASN
	}

	// Устанавливаем время создания, если не указано
	if route.CreatedAt.IsZero() {
		route.CreatedAt = time.Now()
	}

	// Создаем ASN маршрут в базе данных
	if err := s.repo.Route().CreateASN(ctx, route); err != nil {
		return fmt.Errorf("failed to create ASN route in database: %w", err)
	}

	return nil
}

// GetASNRouteByID получает ASN маршрут по ID
func (s *RouteService) GetASNRouteByID(ctx context.Context, id int64) (*models.ASNRoute, error) {
	route, err := s.repo.Route().GetASNByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get ASN route by ID: %w", err)
	}
	return route, nil
}

// ListASNRoutes возвращает список ASN маршрутов по типу
func (s *RouteService) ListASNRoutes(ctx context.Context, routeType models.RouteType) ([]*models.ASNRoute, error) {
	routes, err := s.repo.Route().ListASN(ctx, routeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get ASN routes list: %w", err)
	}
	return routes, nil
}

// CreateRouteGroup создает новую группу маршрутов
func (s *RouteService) CreateRouteGroup(ctx context.Context, group *models.RouteGroup) error {
	// Устанавливаем время создания, если не указано
	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now()
	}

	// Создаем группу маршрутов в базе данных
	if err := s.repo.Route().CreateGroup(ctx, group); err != nil {
		return fmt.Errorf("failed to create route group in database: %w", err)
	}

	return nil
}

// GetRouteGroupByID получает группу маршрутов по ID
func (s *RouteService) GetRouteGroupByID(ctx context.Context, id int64) (*models.RouteGroup, error) {
	group, err := s.repo.Route().GetGroupByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get route group by ID: %w", err)
	}
	return group, nil
}

// AddRouteToGroup добавляет маршрут в группу
func (s *RouteService) AddRouteToGroup(ctx context.Context, groupID, routeID int64) error {
	// Проверяем существование группы
	group, err := s.repo.Route().GetGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("route group not found: %w", err)
	}

	// Проверяем существование маршрута
	route, err := s.repo.Route().GetByID(ctx, routeID)
	if err != nil {
		return fmt.Errorf("route not found: %w", err)
	}

	// Логируем действие
	s.logger.Infof("Adding route %s (ID: %d) to group %s (ID: %d)",
		route.Network, route.ID, group.Name, group.ID)

	// Добавляем маршрут в группу
	if err := s.repo.Route().AddRouteToGroup(ctx, groupID, routeID); err != nil {
		return fmt.Errorf("failed to add route to group: %w", err)
	}

	return nil
}

// RemoveRouteFromGroup удаляет маршрут из группы
func (s *RouteService) RemoveRouteFromGroup(ctx context.Context, groupID, routeID int64) error {
	// Удаляем маршрут из группы
	if err := s.repo.Route().RemoveRouteFromGroup(ctx, groupID, routeID); err != nil {
		return fmt.Errorf("failed to remove route from group: %w", err)
	}
	return nil
}

// GetRoutesInGroup возвращает список маршрутов в группе
func (s *RouteService) GetRoutesInGroup(ctx context.Context, groupID int64) ([]*models.Route, error) {
	routes, err := s.repo.Route().GetRoutesInGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes in group: %w", err)
	}
	return routes, nil
}

// AssignRouteToUser связывает маршрут с пользователем
func (s *RouteService) AssignRouteToUser(ctx context.Context, userRoute *models.UserRoute) error {
	// Устанавливаем время создания, если не указано
	if userRoute.CreatedAt.IsZero() {
		userRoute.CreatedAt = time.Now()
	}

	// Связываем маршрут с пользователем
	if err := s.repo.Route().AssignRouteToUser(ctx, userRoute); err != nil {
		return fmt.Errorf("failed to assign route to user: %w", err)
	}
	return nil
}

// GetUserRoutes возвращает список маршрутов пользователя
func (s *RouteService) GetUserRoutes(ctx context.Context, userID int64) ([]*models.Route, error) {
	routes, err := s.repo.Route().GetUserRoutes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user routes: %w", err)
	}
	return routes, nil
}

// AssignGroupToUser связывает группу маршрутов с пользователем
func (s *RouteService) AssignGroupToUser(ctx context.Context, userGroup *models.UserRouteGroup) error {
	// Устанавливаем время создания, если не указано
	if userGroup.CreatedAt.IsZero() {
		userGroup.CreatedAt = time.Now()
	}

	// Связываем группу с пользователем
	if err := s.repo.Route().AssignGroupToUser(ctx, userGroup); err != nil {
		return fmt.Errorf("failed to assign group to user: %w", err)
	}
	return nil
}

// GetUserGroups возвращает список групп маршрутов пользователя
func (s *RouteService) GetUserGroups(ctx context.Context, userID int64) ([]*models.RouteGroup, error) {
	groups, err := s.repo.Route().GetUserGroups(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}
	return groups, nil
}

// UnassignRouteFromUser удаляет связь маршрута с пользователем
func (s *RouteService) UnassignRouteFromUser(ctx context.Context, userID, routeID int64) error {
	// Удаляем связь маршрута с пользователем
	if err := s.repo.Route().UnassignRouteFromUser(ctx, userID, routeID); err != nil {
		return fmt.Errorf("failed to unassign route from user: %w", err)
	}
	return nil
}

// UnassignGroupFromUser удаляет связь группы маршрутов с пользователем
func (s *RouteService) UnassignGroupFromUser(ctx context.Context, userID, groupID int64) error {
	// Удаляем связь группы с пользователем
	if err := s.repo.Route().UnassignGroupFromUser(ctx, userID, groupID); err != nil {
		return fmt.Errorf("failed to unassign group from user: %w", err)
	}
	return nil
}

// GetUserContextKey тип для ключей контекста пользователя
type UserContextKey string

// Константы для ключей контекста
const (
	UserIDKey   UserContextKey = "user_id"
	UserRoleKey UserContextKey = "user_role"
)

// WithUserID добавляет ID пользователя в контекст
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// UserIDFromContext извлекает ID пользователя из контекста
func UserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	return userID, ok
}

// WithUserRole добавляет роль пользователя в контекст
func WithUserRole(ctx context.Context, role models.RoleType) context.Context {
	return context.WithValue(ctx, UserRoleKey, role)
}

// UserRoleFromContext извлекает роль пользователя из контекста
func UserRoleFromContext(ctx context.Context) (models.RoleType, bool) {
	role, ok := ctx.Value(UserRoleKey).(models.RoleType)
	return role, ok
}
