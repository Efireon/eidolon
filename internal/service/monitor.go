package service

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"eidolon/internal/repository"

	"github.com/sirupsen/logrus"
)

// MonitorService предоставляет методы для мониторинга системы
type MonitorService struct {
	repo    repository.Repository
	logger  *logrus.Logger
	vpn     *VPNService
	metrics *SystemMetrics
	mutex   sync.RWMutex
}

// SystemMetrics содержит метрики системы
type SystemMetrics struct {
	StartTime         time.Time
	TotalConnections  int64
	ActiveConnections int
	TotalTraffic      int64
	CPUUsage          float64
	MemoryUsage       uint64
	LastUpdate        time.Time
	ConnectionHistory map[string]int   // количество подключений по дням
	TrafficHistory    map[string]int64 // объем трафика по дням
}

// NewMonitorService создает новый сервис мониторинга
func NewMonitorService(repo repository.Repository, vpn *VPNService, logger *logrus.Logger) *MonitorService {
	return &MonitorService{
		repo:   repo,
		logger: logger,
		vpn:    vpn,
		metrics: &SystemMetrics{
			StartTime:         time.Now(),
			ConnectionHistory: make(map[string]int),
			TrafficHistory:    make(map[string]int64),
		},
	}
}

// Start запускает сервис мониторинга
func (s *MonitorService) Start(ctx context.Context) {
	// Запускаем горутину для периодического обновления метрик
	go s.updateMetrics(ctx)

	s.logger.Info("Monitor service started")
}

// updateMetrics периодически обновляет метрики системы
func (s *MonitorService) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Сразу обновляем метрики
	s.refreshMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshMetrics(ctx)
		}
	}
}

// refreshMetrics обновляет все метрики системы
func (s *MonitorService) refreshMetrics(ctx context.Context) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Получаем активные подключения
	activeConnections, err := s.vpn.GetActiveConnections(ctx)
	if err != nil {
		s.logger.Errorf("Failed to get active connections: %v", err)
	} else {
		s.metrics.ActiveConnections = len(activeConnections)
	}

	// Получаем общий трафик
	s.calculateTotalTraffic(ctx)

	// Обновляем историю подключений и трафика
	s.updateHistory(ctx)

	// Получаем использование CPU и памяти
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	s.metrics.MemoryUsage = memStats.Alloc

	// Фиксируем время последнего обновления
	s.metrics.LastUpdate = time.Now()

	s.logger.Debug("System metrics refreshed")
}

// calculateTotalTraffic рассчитывает общий объем трафика
func (s *MonitorService) calculateTotalTraffic(ctx context.Context) {
	// Получаем список пользователей
	users, err := s.repo.User().List(ctx, 0, 1000)
	if err != nil {
		s.logger.Errorf("Failed to get users: %v", err)
		return
	}

	var totalTraffic int64
	for _, user := range users {
		// Получаем трафик пользователя
		userTraffic, err := s.repo.Traffic().GetTotalUserTraffic(ctx, user.ID)
		if err != nil {
			s.logger.Warnf("Failed to get traffic for user %s: %v", user.Username, err)
			continue
		}
		totalTraffic += userTraffic
	}

	s.metrics.TotalTraffic = totalTraffic
}

// updateHistory обновляет историю подключений и трафика
func (s *MonitorService) updateHistory(ctx context.Context) {
	// Текущий день
	today := time.Now().Format("2006-01-02")

	// Обновляем счетчик подключений за сегодня
	s.metrics.ConnectionHistory[today] = s.metrics.ActiveConnections

	// Получаем трафик за сегодня
	from := time.Now().Truncate(24 * time.Hour)
	to := time.Now()

	// Получаем список пользователей
	users, err := s.repo.User().List(ctx, 0, 1000)
	if err != nil {
		s.logger.Errorf("Failed to get users: %v", err)
		return
	}

	var todayTraffic int64
	for _, user := range users {
		// Получаем трафик пользователя за сегодня
		userTraffic, err := s.repo.Traffic().GetUserTraffic(ctx, user.ID, from.Unix(), to.Unix())
		if err != nil {
			s.logger.Warnf("Failed to get traffic for user %s: %v", user.Username, err)
			continue
		}

		for _, traffic := range userTraffic {
			todayTraffic += traffic.Bytes
		}
	}

	s.metrics.TrafficHistory[today] = todayTraffic
}

// GetMetrics возвращает текущие метрики системы
func (s *MonitorService) GetMetrics() *SystemMetrics {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Возвращаем копию метрик для безопасности
	return &SystemMetrics{
		StartTime:         s.metrics.StartTime,
		TotalConnections:  s.metrics.TotalConnections,
		ActiveConnections: s.metrics.ActiveConnections,
		TotalTraffic:      s.metrics.TotalTraffic,
		CPUUsage:          s.metrics.CPUUsage,
		MemoryUsage:       s.metrics.MemoryUsage,
		LastUpdate:        s.metrics.LastUpdate,
		ConnectionHistory: copyStringIntMap(s.metrics.ConnectionHistory),
		TrafficHistory:    copyStringInt64Map(s.metrics.TrafficHistory),
	}
}

// GetSystemStatus возвращает текущий статус системы в виде строки
func (s *MonitorService) GetSystemStatus() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.metrics.StartTime)

	status := fmt.Sprintf("Статус системы:\n\n"+
		"Время работы: %s\n"+
		"Активных подключений: %d\n"+
		"Общий трафик: %s\n"+
		"Использование памяти: %s\n"+
		"Последнее обновление: %s",
		formatDuration(uptime),
		s.metrics.ActiveConnections,
		formatBytes(s.metrics.TotalTraffic),
		formatBytes(int64(s.metrics.MemoryUsage)),
		s.metrics.LastUpdate.Format("02.01.2006 15:04:05"),
	)

	return status
}

// Вспомогательные функции

// copyStringIntMap создает копию map[string]int
func copyStringIntMap(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyStringInt64Map создает копию map[string]int64
func copyStringInt64Map(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// formatDuration форматирует длительность в читаемый формат
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d дн. %d ч. %d мин.", days, hours, minutes)
	}

	return fmt.Sprintf("%d ч. %d мин.", hours, minutes)
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
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
