package openconnect

import (
	"eidolonVPN/internal/config"
	"eidolonVPN/internal/config/structures"
	"eidolonVPN/internal/errors/handlers"
	"fmt"
	"os"
	"strings"
)

// GenerateOCconfig генерирует файл конфигурации ocserv на основе YAML
func GenerateOCconfig(sourcePath string, targetPath string) error {
	var ocConfig structures.OpenConnectConfig

	// Используем LoadConfig
	// Передаем название файла и путь для поиска
	err := config.LoadConfig("openconnect", []string{sourcePath}, &ocConfig)
	if err != nil {
		return handlers.OpenConnectConfigErrHandler(sourcePath, err)
	}

	// Формируем содержимое файла ocserv
	configContent := generateOCservConfig(ocConfig)

	// Записываем файл
	return os.WriteFile(targetPath, []byte(configContent), 0644)
}

// Генерация конфигурации ocserv.conf
func generateOCservConfig(config structures.OpenConnectConfig) string {
	var content string

	// Основные параметры сервера
	content += fmt.Sprintf("listen-host = %s\n", config.Server)
	content += fmt.Sprintf("tcp-port = %d\n", config.Port)

	if config.Protocol == "udp" {
		content += fmt.Sprintf("udp-port = %d\n", config.Port)
	}

	// Интерфейс
	content += fmt.Sprintf("device = %s\n", config.Interface)

	// Безопасность
	if len(config.Security.AllowedCiphers) > 0 {
		content += fmt.Sprintf("tls-priorities = %s\n",
			strings.Join(config.Security.AllowedCiphers, ":"))
	}

	if config.Security.CAPath != "" {
		content += fmt.Sprintf("ca-cert = %s\n", config.Security.CAPath)
	}

	// Сетевые настройки
	content += fmt.Sprintf("mtu = %d\n", config.Network.MTU)

	if len(config.Network.DNSServers) > 0 {
		content += fmt.Sprintf("dns = %s\n",
			strings.Join(config.Network.DNSServers, ", "))
	}

	if len(config.Network.SearchDomains) > 0 {
		content += fmt.Sprintf("search-domains = %s\n",
			strings.Join(config.Network.SearchDomains, ", "))
	}

	// Маршруты
	for _, route := range config.Network.Routes {
		content += fmt.Sprintf("route = %s\n", route)
	}

	for _, exclude := range config.Network.ExcludeRoutes {
		content += fmt.Sprintf("no-route = %s\n", exclude)
	}

	// Отладка
	content += fmt.Sprintf("log-level = %d\n", config.Debug.Verbose)
	if config.Debug.LogFile != "" {
		content += fmt.Sprintf("log-file = %s\n", config.Debug.LogFile)
	}

	return content
}

// CheckOCconfig проверяет существование и валидность конфигурации
func CheckOCconfig(configPath string) (bool, error) {
	// Проверяем существование файла
	// Сравниваем с эталонной конфигурацией
	// Возвращаем результат
	return false, nil
}

// SearchOCconfig ищет файл конфигурации по указанному пути
func SearchOCconfig(searchPath string) (string, error) {
	// Ищем файл в указанной директории
	// Возвращаем путь или ошибку
	return "", os.ErrNotExist
}
