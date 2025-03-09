package api

import (
	"context"
	"net/http"
	"time"

	"eidolon/internal/models"
	"eidolon/internal/service"

	"github.com/sirupsen/logrus"
)

// Server представляет HTTP-сервер API
type Server struct {
	server          *http.Server
	handler         *Handler
	logger          *logrus.Logger
	shutdownTimeout time.Duration
}

// ServerConfig содержит конфигурацию сервера API
type ServerConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// NewServer создает новый экземпляр сервера API
func NewServer(
	config ServerConfig,
	authService *service.AuthService,
	inviteService *service.InviteService,
	vpnService *service.VPNService,
	logger *logrus.Logger,
) *Server {
	// Создаем обработчик
	handler := NewHandler(authService, inviteService, vpnService, logger)

	// Создаем роутер
	router := http.NewServeMux()

	// Настраиваем маршруты
	// Оборачиваем все маршруты в CORS-middleware
	corsRouter := WithCORS(router)

	// Публичные маршруты
	router.HandleFunc("/api/auth/register", handler.RegisterUser)
	router.HandleFunc("/api/auth/login", handler.Login)
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Защищенные маршруты
	router.HandleFunc("/api/user/info", handler.authMiddleware(handler.GetUserInfo))
	router.HandleFunc("/api/user/routes", handler.authMiddleware(handler.GetUserRoutes))
	router.HandleFunc("/api/user/routes/add", handler.authMiddleware(handler.AddUserRoute))
	router.HandleFunc("/api/user/routes/remove", handler.authMiddleware(handler.RemoveUserRoute))
	router.HandleFunc("/api/user/traffic", handler.authMiddleware(handler.GetUserTraffic))
	router.HandleFunc("/api/user/traffic/total", handler.authMiddleware(handler.GetTotalUserTraffic))
	router.HandleFunc("/api/user/config", handler.authMiddleware(handler.GetUserConfig))

	// Маршруты только для админов
	router.HandleFunc("/api/routes/create", handler.authMiddleware(handler.checkRole(models.RoleAdmin, handler.CreateRoute)))

	// Создаем HTTP-сервер с CORS-middleware
	server := &http.Server{
		Addr:         config.Addr,
		Handler:      WithCORS(router),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return &Server{
		server:          server,
		handler:         handler,
		logger:          logger,
		shutdownTimeout: config.ShutdownTimeout,
	}
}

// Start запускает сервер API
func (s *Server) Start(ctx context.Context) error {
	s.logger.Infof("Starting API server on %s", s.server.Addr)

	// Запускаем сервер в отдельной горутине
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("API server error: %v", err)
		}
	}()

	// Ожидаем завершение контекста
	<-ctx.Done()

	// Останавливаем сервер
	return s.Stop(context.Background())
}

// Stop останавливает сервер API
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API server...")

	// Создаем контекст с таймаутом для плавного завершения
	shutdownCtx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

// WithCORS добавляет CORS-заголовки к ответам
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Устанавливаем CORS-заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Для preflight-запросов сразу возвращаем ответ
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Продолжаем выполнение цепочки обработчиков
		next.ServeHTTP(w, r)
	})
}
