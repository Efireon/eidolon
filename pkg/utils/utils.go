package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
)

// GenerateRandomBytes генерирует случайные байты указанной длины
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// GenerateRandomString генерирует случайную строку указанной длины
func GenerateRandomString(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}
	return string(bytes), nil
}

// GenerateToken генерирует токен в формате base64
func GenerateToken(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidateCIDR проверяет, что строка является корректным CIDR
func ValidateCIDR(cidr string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR format: %w", err)
	}
	return ipNet.String(), nil
}

// FormatTraffic форматирует количество байт в человекочитаемый формат
func FormatTraffic(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
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

// GetTimeRangeFromPeriod возвращает временной диапазон для указанного периода
func GetTimeRangeFromPeriod(period string) (time.Time, time.Time) {
	now := time.Now()
	var from time.Time

	switch strings.ToLower(period) {
	case "day":
		from = now.Add(-24 * time.Hour)
	case "week":
		from = now.AddDate(0, 0, -7)
	case "month":
		from = now.AddDate(0, -1, 0)
	case "year":
		from = now.AddDate(-1, 0, 0)
	default:
		from = now.AddDate(0, 0, -7) // по умолчанию неделя
	}

	return from, now
}

// ParseASNList парсит список ASN из строки, разделенной запятыми
func ParseASNList(asnString string) ([]int, error) {
	var asnList []int

	// Если строка пуста, возвращаем пустой список
	if asnString == "" {
		return asnList, nil
	}

	// Разделяем строку по запятым
	asnStrings := strings.Split(asnString, ",")

	for _, asnStr := range asnStrings {
		// Удаляем лишние пробелы
		asnStr = strings.TrimSpace(asnStr)

		// Преобразуем строку в число
		var asn int
		_, err := fmt.Sscanf(asnStr, "%d", &asn)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN format: %w", err)
		}

		asnList = append(asnList, asn)
	}

	return asnList, nil
}
