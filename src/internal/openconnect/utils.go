package openconnect

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"eidolonVPN/internal/config"
	"eidolonVPN/internal/config/structures"
	"eidolonVPN/internal/errors"
	"eidolonVPN/internal/errors/handlers"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	// Авторизация
	if strings.Contains(config.Security.Auth, "pam") {
		content += fmt.Sprintf("enable-auth = \"%s\"\n", "gssapi")
		content += fmt.Sprintf("auth = \"%s\"\n", config.Security.Auth)
	} else {
		content += fmt.Sprintf("auth = \"%s\"\n", config.Security.Auth)
	}

	content += fmt.Sprintf("tcp-port = %d\n", config.Port)

	if config.Protocol == "udp" {
		content += fmt.Sprintf("udp-port = %d\n", config.Port)
	}

	// Интерфейс
	content += fmt.Sprintf("device = %s\n", config.Interface)

	// Сокет
	content += fmt.Sprintf("socket-file = \"%s\"\n", config.Socket)

	// Безопасность
	if len(config.Security.AllowedCiphers) > 0 {
		content += fmt.Sprintf("tls-priorities = %s\n",
			strings.Join(config.Security.AllowedCiphers, ":"))
	}

	if config.Security.CAPath != "" {
		content += fmt.Sprintf("server-cert = %s%s\n", config.Security.CAPath, config.Security.CACert)
	}
	if config.Security.CAPath != "" {
		content += fmt.Sprintf("server-key = %s%s\n", config.Security.CAPath, config.Security.CAKey)
	}

	// Сетевые настройки
	content += fmt.Sprintf("default-domain = \"%s\"\n", config.Server)

	content += fmt.Sprintf("ipv4-network = %s\n", config.Network.LAN)
	content += fmt.Sprintf("ipv4-netmask = %s\n", config.Network.LANMask)

	content += fmt.Sprintf("mtu = %d\n", config.Network.MTU)

	if len(config.Network.DNSServers) > 0 {
		content += fmt.Sprintf("dns = %s\n",
			strings.Join(config.Network.DNSServers, ", "))
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

// Генерация ssl сертификата
func GenerateSSLcert(path string) (string, string, error) {
	// Загрузка конфигурации
	var ocConfig structures.OpenConnectConfig
	err := config.LoadConfig("openconnect", []string{"/eidolon/service/config"}, &ocConfig)
	if err != nil {
		return "", "", err
	}

	// Проверяем, существуют ли уже сертификаты
	certPath := filepath.Join(path, ocConfig.Security.CACert)
	keyPath := filepath.Join(path, ocConfig.Security.CAKey)

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return certPath, keyPath, nil // Оба файла существуют
		}
	}

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", "", err
	}

	// Генерируем приватный ключ RSA
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Сериализуем приватный ключ в PEM
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", err
	}
	defer keyOut.Close()

	err = pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err != nil {
		return "", "", err
	}

	// Создаем шаблон сертификата
	notBefore := time.Now()
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour) // 10 лет

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{ocConfig.Name},
			CommonName:   ocConfig.Server,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost", ocConfig.Server},
	}

	// Самоподписанный сертификат (CA и серверный сертификат в одном)
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}

	// Сериализуем сертификат в PEM
	certOut, err := os.Create(certPath)
	if err != nil {
		return "", "", err
	}
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return "", "", err
	}

	// Установка прав доступа
	if err := os.Chmod(keyPath, 0600); err != nil {
		return "", "", err
	}

	if err := os.Chmod(certPath, 0644); err != nil {
		return "", "", err
	}

	return certPath, keyPath, nil
}
