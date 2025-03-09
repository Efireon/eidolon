package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"eidolon/internal/models"
	"eidolon/internal/service"

	"github.com/sirupsen/logrus"
)

// Handler содержит обработчики HTTP-запросов
type Handler struct {
	authService   *service.AuthService
	inviteService *service.InviteService
	vpnService    *service.VPNService
	logger        *logrus.Logger
}

// NewHandler создает новый экземпляр Handler
func NewHandler(authService *service.AuthService, inviteService *service.InviteService, vpnService *service.VPNService, logger *logrus.Logger) *Handler {
	return &Handler{
		authService:   authService,
		inviteService: inviteService,
		vpnService:    vpnService,
		logger:        logger,
	}
}

// response представляет общий формат ответа API
type response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// sendResponse отправляет JSON-ответ
func (h *Handler) sendResponse(w http.ResponseWriter, status int, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Errorf("Failed to encode response: %v", err)
	}
}

// authMiddleware проверяет JWT-токен
func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем токен из заголовка Authorization
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			h.sendResponse(w, http.StatusUnauthorized, response{
				Success: false,
				Error:   "No authorization token provided",
			})
			return
		}

		// Удаляем префикс "Bearer " если он есть
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		// Проверяем токен
		claims, err := h.authService.ValidateToken(tokenString)
		if err != nil {
			h.sendResponse(w, http.StatusUnauthorized, response{
				Success: false,
				Error:   "Invalid token: " + err.Error(),
			})
			return
		}

		// Добавляем данные пользователя в контекст запроса
		ctx := r.Context()
		ctx = service.WithUserID(ctx, claims.UserID)
		ctx = service.WithUserRole(ctx, claims.Role)

		// Вызываем следующий обработчик с обновленным контекстом
		next(w, r.WithContext(ctx))
	}
}

// checkRole проверяет, имеет ли пользователь указанную роль
func (h *Handler) checkRole(role models.RoleType, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userRole, ok := service.UserRoleFromContext(r.Context())
		if !ok {
			h.sendResponse(w, http.StatusUnauthorized, response{
				Success: false,
				Error:   "User role not found in context",
			})
			return
		}

		if !h.authService.CheckUserPermission(userRole, role) {
			h.sendResponse(w, http.StatusForbidden, response{
				Success: false,
				Error:   "Insufficient permissions",
			})
			return
		}

		next(w, r)
	}
}

// RegisterUser обрабатывает запрос на регистрацию пользователя с помощью инвайт-кода
func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	// Декодируем запрос
	var req struct {
		Username   string `json:"username"`
		InviteCode string `json:"invite_code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Создаем временного пользователя
	user := &models.User{
		Username:  req.Username,
		CreatedAt: time.Now(),
	}

	// Проверяем инвайт-код и регистрируем пользователя
	err := h.inviteService.UseInviteCode(r.Context(), req.InviteCode, user)
	if err != nil {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Failed to register: " + err.Error(),
		})
		return
	}

	// Создаем сертификат для пользователя
	cert, err := h.vpnService.CreateUserCertificate(r.Context(), user)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to create certificate: " + err.Error(),
		})
		return
	}

	// Генерируем JWT-токен
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to generate token: " + err.Error(),
		})
		return
	}

	// Отправляем успешный ответ с данными пользователя
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Message: "User registered successfully",
		Data: map[string]interface{}{
			"user_id":     user.ID,
			"username":    user.Username,
			"role":        user.Role,
			"token":       token,
			"certificate": cert,
		},
	})
}

// Login обрабатывает запрос на аутентификацию
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	// В этом примере мы предполагаем, что аутентификация происходит по сертификату
	var req struct {
		Certificate string `json:"certificate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Аутентификация по сертификату
	user, err := h.authService.AuthenticateWithCertificate(r.Context(), req.Certificate)
	if err != nil {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "Authentication failed: " + err.Error(),
		})
		return
	}

	// Генерируем JWT-токен
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to generate token: " + err.Error(),
		})
		return
	}

	// Отправляем успешный ответ с токеном
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Message: "Authentication successful",
		Data: map[string]interface{}{
			"user_id":  user.ID,
			"username": user.Username,
			"role":     user.Role,
			"token":    token,
		},
	})
}

// GetUserInfo возвращает информацию о пользователе
func (h *Handler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Получаем пользователя из базы данных
	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.sendResponse(w, http.StatusNotFound, response{
			Success: false,
			Error:   "User not found: " + err.Error(),
		})
		return
	}

	// Отправляем информацию о пользователе
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Data: map[string]interface{}{
			"user_id":       user.ID,
			"username":      user.Username,
			"role":          user.Role,
			"created_at":    user.CreatedAt,
			"last_login_at": user.LastLoginAt,
		},
	})
}

// GetUserRoutes возвращает маршруты, доступные пользователю
func (h *Handler) GetUserRoutes(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Получаем маршруты пользователя
	routes, err := h.vpnService.GetUserRoutes(r.Context(), userID)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to get routes: " + err.Error(),
		})
		return
	}

	// Отправляем список маршрутов
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Data:    routes,
	})
}

// AddUserRoute добавляет маршрут для пользователя
func (h *Handler) AddUserRoute(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Декодируем запрос
	var req struct {
		RouteID int64 `json:"route_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Добавляем маршрут пользователю
	err := h.vpnService.AddUserRoute(r.Context(), userID, req.RouteID)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to add route: " + err.Error(),
		})
		return
	}

	// Отправляем успешный ответ
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Message: "Route added successfully",
	})
}

// RemoveUserRoute удаляет маршрут пользователя
func (h *Handler) RemoveUserRoute(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Получаем ID маршрута из URL
	routeIDStr := r.URL.Query().Get("route_id")
	if routeIDStr == "" {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Route ID is required",
		})
		return
	}

	routeID, err := strconv.ParseInt(routeIDStr, 10, 64)
	if err != nil {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Invalid route ID",
		})
		return
	}

	// Удаляем маршрут
	err = h.vpnService.RemoveUserRoute(r.Context(), userID, routeID)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to remove route: " + err.Error(),
		})
		return
	}

	// Отправляем успешный ответ
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Message: "Route removed successfully",
	})
}

// CreateRoute создает новый маршрут (только для админов)
func (h *Handler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Декодируем запрос
	var req struct {
		Network     string           `json:"network"`
		Description string           `json:"description"`
		Type        models.RouteType `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Создаем новый маршрут
	route := &models.Route{
		Network:     req.Network,
		Description: req.Description,
		Type:        req.Type,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
	}

	err := h.vpnService.CreateRoute(r.Context(), route)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to create route: " + err.Error(),
		})
		return
	}

	// Отправляем успешный ответ
	h.sendResponse(w, http.StatusCreated, response{
		Success: true,
		Message: "Route created successfully",
		Data:    route,
	})
}

// GetUserTraffic возвращает статистику трафика пользователя
func (h *Handler) GetUserTraffic(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Получаем параметры запроса
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to int64
	var err error

	if fromStr != "" {
		from, err = strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			h.sendResponse(w, http.StatusBadRequest, response{
				Success: false,
				Error:   "Invalid 'from' timestamp",
			})
			return
		}
	} else {
		// По умолчанию, последние 30 дней
		from = time.Now().AddDate(0, 0, -30).Unix()
	}

	if toStr != "" {
		to, err = strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			h.sendResponse(w, http.StatusBadRequest, response{
				Success: false,
				Error:   "Invalid 'to' timestamp",
			})
			return
		}
	} else {
		to = time.Now().Unix()
	}

	// Получаем статистику трафика
	traffic, err := h.vpnService.GetUserTraffic(r.Context(), userID, from, to)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to get traffic statistics: " + err.Error(),
		})
		return
	}

	// Отправляем статистику трафика
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Data:    traffic,
	})
}

// GetTotalUserTraffic возвращает общий объем трафика пользователя
func (h *Handler) GetTotalUserTraffic(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Получаем общий объем трафика
	total, err := h.vpnService.GetTotalUserTraffic(r.Context(), userID)
	if err != nil {
		h.sendResponse(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "Failed to get total traffic: " + err.Error(),
		})
		return
	}

	// Отправляем общий объем трафика
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Data: map[string]interface{}{
			"total_traffic": total,
			"formatted":     formatBytes(total),
		},
	})
}

// GetUserConfig возвращает конфигурацию OpenConnect для пользователя
func (h *Handler) GetUserConfig(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из контекста
	userID, ok := service.UserIDFromContext(r.Context())
	if !ok {
		h.sendResponse(w, http.StatusUnauthorized, response{
			Success: false,
			Error:   "User ID not found in context",
		})
		return
	}

	// Получаем пользователя
	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.sendResponse(w, http.StatusNotFound, response{
			Success: false,
			Error:   "User not found: " + err.Error(),
		})
		return
	}

	// Проверяем, что у пользователя есть сертификат
	if user.Certificate == "" {
		h.sendResponse(w, http.StatusBadRequest, response{
			Success: false,
			Error:   "User has no certificate",
		})
		return
	}

	// Генерируем конфигурацию
	config := generateOpenConnectConfig(user)

	// Отправляем конфигурацию
	h.sendResponse(w, http.StatusOK, response{
		Success: true,
		Data: map[string]interface{}{
			"config": config,
		},
	})
}

// generateOpenConnectConfig генерирует конфигурационный файл для клиента OpenConnect
func generateOpenConnectConfig(user *models.User) string {
	return `# Eidolon VPN configuration for OpenConnect
# Username: ` + user.Username + `
# Generated: ` + time.Now().Format(time.RFC3339) + `

server=vpn.example.com
port=443
protocol=tcp
user=` + user.Username + `
authgroup=Eidolon

# Certificate
` + user.Certificate
}

// formatBytes форматирует количество байт в читаемый формат
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return strconv.FormatFloat(float64(bytes)/TB, 'f', 2, 64) + " TB"
	case bytes >= GB:
		return strconv.FormatFloat(float64(bytes)/GB, 'f', 2, 64) + " GB"
	case bytes >= MB:
		return strconv.FormatFloat(float64(bytes)/MB, 'f', 2, 64) + " MB"
	case bytes >= KB:
		return strconv.FormatFloat(float64(bytes)/KB, 'f', 2, 64) + " KB"
	default:
		return strconv.FormatInt(bytes, 10) + " B"
	}
}
