package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yourusername/eidolon/internal/models"
	"github.com/yourusername/eidolon/internal/repository"
	"github.com/yourusername/eidolon/internal/vpn"
)

// VPNService предоставляет методы для управления VPN
type VPNService struct {
	repo              repository.Repository
	vpnServer         *vpn.OpenConnectServer
	certManager       *vpn.CertificateManager
	logger            *logrus.Logger
	defaultRoutes     []string
	defaultAsnRoutes  []int
	activeConnections map[int64]string // mapping userID -> username
	mutex             sync.RWMutex
}

// NewVPNService создает новый сервис управления VPN
func NewVPNService(
	repo repository.Repository,
	vpnServer *vpn.OpenConnectServer,
	certManager *vpn.CertificateManager,
	logger *logrus.Logger,
	defaultRoutes []string,
	defaultAsnRoutes []int,
) *VPNService {
	return &VPNService{
		repo:              repo,
		vpnServer:         vpnServer,
		certManager:       certManager,
		logger:            logger,
		defaultRoutes:     defaultRoutes,
		defaultAsnRoutes:  defaultAsnRoutes,
		activeConnections: make(map[int64]string),
	}
}

// Start запускает VPN сервер
func (s *VPNService) Start(ctx context.Context) error {
	// Загружаем маршруты по умолчанию
	for _, route := range s.defaultRoutes {
		if err := s.vpnServer.AddRoute(route); err != nil {
			s.logger.Warnf("Failed to add default route %s: %v", route, err)
		}
	}

	// Загружаем ASN маршруты по умолчанию
	for _, asn := range s.defaultAsnRoutes {
		s.vpnServer.AddASNRoute(asn)
	}

	// Загружаем дополнительные маршруты из базы данных
	routes, err := s.repo.Route().List(ctx, models.RouteTypeDefault)
	if err != nil {
		s.logger.Warnf("Failed to load default routes from database: %v", err)
	} else {
		for _, route := range routes {
			if err := s.vpnServer.AddRoute(route.Network); err != nil {
				s.logger.Warnf("Failed to add route %s: %v", route.Network, err)
			}
		}
	}

	// Загружаем заблокированные маршруты
	blockedRoutes, err := s.repo.Route().List(ctx, models.RouteTypeBlock)
	if err != nil {
		s.logger.Warnf("Failed to load blocked routes from database: %v", err)
	} else {
		for _, route := range blockedRoutes {
			if err := s.vpnServer.BlockRoute(route.Network); err != nil {
				s.logger.Warnf("Failed to block route %s: %v", route.Network, err)
			}
		}
	}

	// Запускаем сервер
	if err := s.vpnServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start VPN server: %w", err)
	}

	// Запускаем периодическое обновление статистики трафика
	go s.monitorTraffic(ctx)

	return nil
}

// Stop останавливает VPN сервер
func (s *VPNService) Stop() error {
	return s.vpnServer.Stop()
}

// CreateUserCertificate создает сертификат для пользователя
func (s *VPNService) CreateUserCertificate(ctx context.Context, user *models.User) (string, error) {
	// Проверяем, что пользователь существует и имеет допустимую роль
	if user.ID == 0 {
		return "", fmt.Errorf("user not found")
	}

	// Создаем сертификат
	options := vpn.CertOptions{
		CommonName:   user.Username,
		Organization: "Eidolon VPN",
		Country:      "RU",
		Locality:     "Internet",
		ValidForDays: 365, // Сертификат действителен 1 год
	}

	certPEM, err := s.certManager.CreateClientCertificate(user.Username, options)
	if err != nil {
		return "", fmt.Errorf("failed to create client certificate: %w", err)
	}

	// Обновляем пользователя в базе данных
	user.Certificate = certPEM
	if err := s.repo.User().Update(ctx, user); err != nil {
		return "", fmt.Errorf("failed to update user with certificate: %w", err)
	}

	return certPEM, nil
}

// GetUserRoutes возвращает маршруты, доступные пользователю
func (s *VPNService) GetUserRoutes(ctx context.Context, userID int64) ([]*models.Route, error) {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Получаем индивидуальные маршруты
	routes, err := s.repo.Route().GetUserRoutes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user routes: %w", err)
	}

	// Получаем группы маршрутов
	groups, err := s.repo.Route().GetUserGroups(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user route groups: %w", err)
	}

	// Добавляем маршруты из групп
	for _, group := range groups {
		groupRoutes, err := s.repo.Route().GetRoutesInGroup(ctx, group.ID)
		if err != nil {
			s.logger.Warnf("Failed to get routes in group %d: %v", group.ID, err)
			continue
		}
		routes = append(routes, groupRoutes...)
	}

	// Для пользователей с ролью "vassal" добавляем только маршруты по умолчанию,
	// если у них еще нет индивидуальных маршрутов
	if user.Role == models.RoleVassal && len(routes) == 0 {
		defaultRoutes, err := s.repo.Route().List(ctx, models.RouteTypeDefault)
		if err != nil {
			return nil, fmt.Errorf("failed to get default routes: %w", err)
		}
		routes = append(routes, defaultRoutes...)
	}

	return routes, nil
}

// AddUserRoute добавляет маршрут для пользователя
func (s *VPNService) AddUserRoute(ctx context.Context, userID int64, routeID int64) error {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем, что пользователь имеет право добавлять маршруты
	userLimits := user.GetRoleLimits()
	if !userLimits.CanAddRoutes {
		return fmt.Errorf("user does not have permission to add routes")
	}

	// Добавляем маршрут
	userRoute := &models.UserRoute{
		UserID:    userID,
		RouteID:   routeID,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	return s.repo.Route().AssignRouteToUser(ctx, userRoute)
}

// RemoveUserRoute удаляет маршрут для пользователя
func (s *VPNService) RemoveUserRoute(ctx context.Context, userID int64, routeID int64) error {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем, что пользователь имеет право удалять маршруты
	userLimits := user.GetRoleLimits()
	if !userLimits.CanAddRoutes {
		return fmt.Errorf("user does not have permission to manage routes")
	}

	return s.repo.Route().UnassignRouteFromUser(ctx, userID, routeID)
}

// AddUserRouteGroup добавляет группу маршрутов для пользователя
func (s *VPNService) AddUserRouteGroup(ctx context.Context, userID int64, groupID int64) error {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем, что пользователь имеет право добавлять маршруты
	userLimits := user.GetRoleLimits()
	if !userLimits.CanAddRoutes {
		return fmt.Errorf("user does not have permission to add routes")
	}

	// Добавляем группу маршрутов
	userGroup := &models.UserRouteGroup{
		UserID:    userID,
		GroupID:   groupID,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	return s.repo.Route().AssignGroupToUser(ctx, userGroup)
}

// RemoveUserRouteGroup удаляет группу маршрутов для пользователя
func (s *VPNService) RemoveUserRouteGroup(ctx context.Context, userID int64, groupID int64) error {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем, что пользователь имеет право удалять маршруты
	userLimits := user.GetRoleLimits()
	if !userLimits.CanAddRoutes {
		return fmt.Errorf("user does not have permission to manage routes")
	}

	return s.repo.Route().UnassignGroupFromUser(ctx, userID, groupID)
}

// CreateRoute создает новый маршрут
func (s *VPNService) CreateRoute(ctx context.Context, route *models.Route) error {
	// Создаем маршрут в базе данных
	err := s.repo.Route().Create(ctx, route)
	if err != nil {
		return fmt.Errorf("failed to create route in database: %w", err)
	}

	// Если это маршрут по умолчанию или пользовательский, добавляем его в VPN сервер
	if route.Type == models.RouteTypeDefault || route.Type == models.RouteTypeCustom {
		if err := s.vpnServer.AddRoute(route.Network); err != nil {
			s.logger.Warnf("Failed to add route %s to VPN server: %v", route.Network, err)
		}
	} else if route.Type == models.RouteTypeBlock {
		// Если это заблокированный маршрут, добавляем его в блок-лист
		if err := s.vpnServer.BlockRoute(route.Network); err != nil {
			s.logger.Warnf("Failed to block route %s in VPN server: %v", route.Network, err)
		}
	}

	return nil
}

// CreateASNRoute создает новый ASN маршрут
func (s *VPNService) CreateASNRoute(ctx context.Context, route *models.ASNRoute) error {
	// Создаем ASN маршрут в базе данных
	err := s.repo.Route().CreateASN(ctx, route)
	if err != nil {
		return fmt.Errorf("failed to create ASN route in database: %w", err)
	}

	// Если это маршрут по умолчанию или пользовательский, добавляем его в VPN сервер
	if route.Type == models.RouteTypeDefault || route.Type == models.RouteTypeCustom {
		s.vpnServer.AddASNRoute(route.ASN)
	}

	return nil
}

// CreateRouteGroup создает новую группу маршрутов
func (s *VPNService) CreateRouteGroup(ctx context.Context, group *models.RouteGroup) error {
	return s.repo.Route().CreateGroup(ctx, group)
}

// AddRouteToGroup добавляет маршрут в группу
func (s *VPNService) AddRouteToGroup(ctx context.Context, groupID, routeID int64) error {
	return s.repo.Route().AddRouteToGroup(ctx, groupID, routeID)
}

// RemoveRouteFromGroup удаляет маршрут из группы
func (s *VPNService) RemoveRouteFromGroup(ctx context.Context, groupID, routeID int64) error {
	return s.repo.Route().RemoveRouteFromGroup(ctx, groupID, routeID)
}

// GetUserTraffic возвращает статистику трафика пользователя
func (s *VPNService) GetUserTraffic(ctx context.Context, userID int64, from, to int64) ([]*models.UserTraffic, error) {
	return s.repo.Traffic().GetUserTraffic(ctx, userID, from, to)
}

// GetTotalUserTraffic возвращает общий объем трафика пользователя
func (s *VPNService) GetTotalUserTraffic(ctx context.Context, userID int64) (int64, error) {
	return s.repo.Traffic().GetTotalUserTraffic(ctx, userID)
}

// DisconnectUser отключает пользователя от VPN
func (s *VPNService) DisconnectUser(ctx context.Context, userID int64) error {
	s.mutex.RLock()
	username, exists := s.activeConnections[userID]
	s.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("user is not connected")
	}

	return s.vpnServer.DisconnectUser(username)
}

// GetActiveConnections возвращает список активных подключений
func (s *VPNService) GetActiveConnections(ctx context.Context) (map[int64]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Копируем карту для безопасного возврата
	connections := make(map[int64]string, len(s.activeConnections))
	for userID, username := range s.activeConnections {
		connections[userID] = username
	}

	return connections, nil
}

// monitorTraffic периодически обновляет статистику трафика пользователей
func (s *VPNService) monitorTraffic(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateTrafficStats(ctx)
		}
	}
}

// updateTrafficStats обновляет статистику трафика для всех активных подключений
func (s *VPNService) updateTrafficStats(ctx context.Context) {
	// Получаем список активных подключений от VPN сервера
	serverConnections, err := s.vpnServer.GetActiveConnections()
	if err != nil {
		s.logger.Errorf("Failed to get active connections: %v", err)
		return
	}

	// Обновляем локальную карту активных подключений
	s.mutex.Lock()
	s.activeConnections = make(map[int64]string)
	s.mutex.Unlock()

	// Для каждого активного подключения
	for _, username := range serverConnections {
		// Находим пользователя по имени
		user, err := s.repo.User().GetByUsername(ctx, username)
		if err != nil {
			s.logger.Warnf("Failed to find user for connection %s: %v", username, err)
			continue
		}

		// Обновляем карту активных подключений
		s.mutex.Lock()
		s.activeConnections[user.ID] = username
		s.mutex.Unlock()

		// Получаем статистику трафика для пользователя
		bytesIn, bytesOut, err := s.vpnServer.GetUserTraffic(username)
		if err != nil {
			s.logger.Warnf("Failed to get traffic stats for user %s: %v", username, err)
			continue
		}

		// Записываем статистику трафика
		traffic := &models.UserTraffic{
			UserID:    user.ID,
			Bytes:     bytesIn + bytesOut,
			Timestamp: time.Now(),
		}

		err = s.repo.Traffic().LogTraffic(ctx, traffic)
		if err != nil {
			s.logger.Warnf("Failed to log traffic for user %s: %v", username, err)
		}

		// Проверяем лимит трафика
		if user.TrafficLimit > 0 {
			totalTraffic, err := s.repo.Traffic().GetTotalUserTraffic(ctx, user.ID)
			if err != nil {
				s.logger.Warnf("Failed to get total traffic for user %s: %v", username, err)
				continue
			}

			// Если превышен лимит трафика, отключаем пользователя
			if totalTraffic > user.TrafficLimit {
				s.logger.Infof("User %s exceeded traffic limit, disconnecting", username)
				if err := s.vpnServer.DisconnectUser(username); err != nil {
					s.logger.Warnf("Failed to disconnect user %s: %v", username, err)
				}
			}
		}
	}
}
