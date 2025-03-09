package vpn

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// OpenConnectServer представляет OpenConnect VPN сервер
type OpenConnectServer struct {
	cmd            *exec.Cmd
	listenIP       string
	listenPort     int
	certFile       string
	keyFile        string
	caFile         string
	routes         []string
	blockRoutes    []string
	asnRoutes      []int
	blockAsnRoutes []int
	mutex          sync.RWMutex
	logger         *logrus.Logger
}

// NewOpenConnectServer создает новый экземпляр OpenConnect сервера
func NewOpenConnectServer(options ...OpenConnectOption) *OpenConnectServer {
	server := &OpenConnectServer{
		listenIP:   "0.0.0.0",
		listenPort: 443,
		logger:     logrus.New(),
	}

	for _, option := range options {
		option(server)
	}

	return server
}

// OpenConnectOption - опция для конфигурации OpenConnect сервера
type OpenConnectOption func(*OpenConnectServer)

// WithListenIP устанавливает IP адрес для прослушивания
func WithListenIP(ip string) OpenConnectOption {
	return func(s *OpenConnectServer) {
		s.listenIP = ip
	}
}

// WithListenPort устанавливает порт для прослушивания
func WithListenPort(port int) OpenConnectOption {
	return func(s *OpenConnectServer) {
		s.listenPort = port
	}
}

// WithCertificate устанавливает файл сертификата сервера
func WithCertificate(certFile, keyFile string) OpenConnectOption {
	return func(s *OpenConnectServer) {
		s.certFile = certFile
		s.keyFile = keyFile
	}
}

// WithCA устанавливает файл CA сертификата
func WithCA(caFile string) OpenConnectOption {
	return func(s *OpenConnectServer) {
		s.caFile = caFile
	}
}

// WithLogger устанавливает логгер
func WithLogger(logger *logrus.Logger) OpenConnectOption {
	return func(s *OpenConnectServer) {
		s.logger = logger
	}
}

// Start запускает OpenConnect сервер
func (s *OpenConnectServer) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("server is already running")
	}

	args := []string{
		"--listen=" + s.listenIP,
		fmt.Sprintf("--port=%d", s.listenPort),
		"--certificate=" + s.certFile,
		"--key=" + s.keyFile,
	}

	if s.caFile != "" {
		args = append(args, "--cafile="+s.caFile)
	}

	// Настройка сплит-туннелирования
	for _, route := range s.routes {
		args = append(args, "--route="+route)
	}

	for _, route := range s.blockRoutes {
		args = append(args, "--no-route="+route)
	}

	// TODO: Добавить поддержку ASN маршрутов (требуется дополнительная логика)

	s.cmd = exec.CommandContext(ctx, "ocserv", args...)

	// Настройка вывода логов
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Запуск мониторинга логов в отдельных горутинах
	go monitorLogs(stdout, s.logger.Info)
	go monitorLogs(stderr, s.logger.Error)

	// Запуск сервера
	err = s.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start ocserv: %w", err)
	}

	s.logger.Info("OpenConnect server started successfully")

	// Запуск горутины для ожидания завершения процесса
	go func() {
		err := s.cmd.Wait()
		s.mutex.Lock()
		defer s.mutex.Unlock()

		if err != nil && ctx.Err() == nil {
			s.logger.Errorf("OpenConnect server exited with error: %v", err)
		} else {
			s.logger.Info("OpenConnect server stopped")
		}

		s.cmd = nil
	}()

	return nil
}

// Stop останавливает OpenConnect сервер
func (s *OpenConnectServer) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.logger.Info("Stopping OpenConnect server...")

	// Отправляем SIGTERM для плавного завершения
	err := s.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Даем серверу время на корректное завершение
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-done:
		s.logger.Info("OpenConnect server stopped successfully")
	case <-time.After(5 * time.Second):
		s.logger.Warn("OpenConnect server did not stop gracefully, forcing shutdown...")
		err = s.cmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	s.cmd = nil
	return nil
}

// AddRoute добавляет маршрут для проксирования
func (s *OpenConnectServer) AddRoute(cidr string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Проверяем, что CIDR корректный
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %w", err)
	}

	// Проверяем, есть ли уже этот маршрут
	for _, route := range s.routes {
		if route == cidr {
			return nil // Маршрут уже добавлен
		}
	}

	s.routes = append(s.routes, cidr)

	// Если сервер запущен, нужно перезапустить его с новыми настройками
	if s.cmd != nil && s.cmd.Process != nil {
		s.logger.Info("Route added, server restart required")
		// TODO: Реализовать обновление конфигурации без перезапуска, если это возможно
		// Для полного применения маршрутов может потребоваться перезапуск сервера
	}

	return nil
}

// RemoveRoute удаляет маршрут
func (s *OpenConnectServer) RemoveRoute(cidr string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, route := range s.routes {
		if route == cidr {
			// Удаляем элемент, сохраняя порядок
			s.routes = append(s.routes[:i], s.routes[i+1:]...)
			break
		}
	}

	// Если сервер запущен, аналогично может потребоваться перезапуск
	if s.cmd != nil && s.cmd.Process != nil {
		s.logger.Info("Route removed, server restart required")
	}
}

// BlockRoute добавляет маршрут в список блокировок
func (s *OpenConnectServer) BlockRoute(cidr string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Проверяем, что CIDR корректный
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %w", err)
	}

	// Проверяем, есть ли уже этот маршрут в блокированных
	for _, route := range s.blockRoutes {
		if route == cidr {
			return nil // Маршрут уже заблокирован
		}
	}

	s.blockRoutes = append(s.blockRoutes, cidr)

	return nil
}

// UnblockRoute удаляет маршрут из списка блокировок
func (s *OpenConnectServer) UnblockRoute(cidr string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, route := range s.blockRoutes {
		if route == cidr {
			// Удаляем элемент, сохраняя порядок
			s.blockRoutes = append(s.blockRoutes[:i], s.blockRoutes[i+1:]...)
			break
		}
	}
}

// AddASNRoute добавляет маршрут на основе ASN
func (s *OpenConnectServer) AddASNRoute(asn int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Проверяем, есть ли уже этот ASN
	for _, a := range s.asnRoutes {
		if a == asn {
			return // ASN уже добавлен
		}
	}

	s.asnRoutes = append(s.asnRoutes, asn)

	// Для ASN потребуется дополнительная логика, чтобы преобразовать их в CIDR
	// TODO: Реализовать определение CIDR для ASN
}

// RemoveASNRoute удаляет маршрут по ASN
func (s *OpenConnectServer) RemoveASNRoute(asn int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, a := range s.asnRoutes {
		if a == asn {
			// Удаляем элемент, сохраняя порядок
			s.asnRoutes = append(s.asnRoutes[:i], s.asnRoutes[i+1:]...)
			break
		}
	}
}

// GetActiveConnections возвращает активные VPN подключения
func (s *OpenConnectServer) GetActiveConnections() ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Используем occtl для получения информации о подключениях
	cmd := exec.Command("occtl", "show", "users")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get active connections: %w", err)
	}

	// Парсим вывод occtl
	connections := parseOcctlOutput(string(output))
	return connections, nil
}

// DisconnectUser отключает пользователя от VPN
func (s *OpenConnectServer) DisconnectUser(username string) error {
	// Используем occtl для отключения пользователя
	cmd := exec.Command("occtl", "disconnect", "user", username)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to disconnect user %s: %w", username, err)
	}

	return nil
}

// GetUserTraffic возвращает статистику трафика пользователя
func (s *OpenConnectServer) GetUserTraffic(username string) (int64, int64, error) {
	// Используем occtl для получения статистики трафика
	cmd := exec.Command("occtl", "show", "user", username)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get user traffic stats: %w", err)
	}

	// Парсим вывод occtl для получения in/out трафика
	bytesIn, bytesOut := parseOcctlTraffic(string(output))
	return bytesIn, bytesOut, nil
}

// Вспомогательные функции

// monitorLogs читает данные из пайпа и отправляет их в логгер
func monitorLogs(reader io.Reader, logFunc func(args ...interface{})) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logFunc(scanner.Text())
	}
}

// parseOcctlOutput парсит вывод команды occtl show users
func parseOcctlOutput(output string) []string {
	var connections []string
	lines := strings.Split(output, "\n")

	// Пропускаем заголовок
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Разбиваем строку на поля
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			connections = append(connections, fields[0])
		}
	}

	return connections
}

// parseOcctlTraffic парсит вывод команды occtl show user
func parseOcctlTraffic(output string) (int64, int64) {
	var bytesIn, bytesOut int64
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "RX:") {
			// Парсим входящий трафик
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Преобразуем строку в число байт
				byteStr := strings.Replace(parts[1], ",", "", -1)
				bytes, _ := strconv.ParseInt(byteStr, 10, 64)
				bytesIn = bytes
			}
		} else if strings.HasPrefix(line, "TX:") {
			// Парсим исходящий трафик
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Преобразуем строку в число байт
				byteStr := strings.Replace(parts[1], ",", "", -1)
				bytes, _ := strconv.ParseInt(byteStr, 10, 64)
				bytesOut = bytes
			}
		}
	}

	return bytesIn, bytesOut
}
