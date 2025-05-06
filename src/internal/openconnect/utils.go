package openconnect

import (
	"eidolonVPN/internal/config"
	"eidolonVPN/internal/config/structures"
	"eidolonVPN/internal/errors"
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
		return handlers.OpenConnectYamlErrHandler(sourcePath, err)
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
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		return false, handlers.OpenConnectFileErrHandler(configPath, err)
	}

	// Читаем содержимое файла
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, handlers.OpenConnectFileErrHandler(configPath, err)
	}
	content := string(data)

	// Загружаем эталонную YAML конфигурацию
	var ocConfig structures.OpenConnectConfig
	err = config.LoadConfig("openconnect", []string{"/eidolon/service/config"}, &ocConfig)
	if err != nil {
		return false, err
	}

	// Генерируем эталонный конфиг из загруженной конфигурации
	expectedConfig := generateOCservConfig(ocConfig)

	// Сравниваем каждую значимую строку
	expectedLines := strings.Split(expectedConfig, "\n")
	for _, line := range expectedLines {
		if line == "" {
			continue
		}

		// Проверяем наличие этой строки в текущем конфиге
		if !strings.Contains(content, line) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				param := strings.TrimSpace(parts[0])
				return false, errors.CallOpenConnectError(fmt.Sprintf("parameter %s has incorrect value in config", param), nil)
			} else {
				return false, errors.CallOpenConnectError(fmt.Sprintf("required line '%s' not found in config", line), nil)
			}
		}
	}

	return true, nil
}

// SearchOCconfig ищет файл конфигурации в стандартных местах
func SearchOCconfig(searchPath string) (string, error) {
	// Проверяем указанный путь
	if _, err := os.Stat(searchPath); err == nil {
		return searchPath, nil
	}

	// Стандартные места для поиска
	standardPaths := []string{
		"/etc/ocserv/ocserv.conf",
		"/etc/ocserv.conf",
		"/usr/local/etc/ocserv/ocserv.conf",
		"/eidolon/service/config/ocserv.conf",
	}

	// Ищем в стандартных местах
	for _, path := range standardPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Если не нашли, возвращаем ошибку
	return "", os.ErrNotExist
}
