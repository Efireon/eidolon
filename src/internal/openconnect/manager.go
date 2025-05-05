package openconnect

import (
	"os"
)

// GenerateOCconfig генерирует файл конфигурации ocserv на основе нашего YAML
func GenerateOCconfig(sourcePath string, targetPath string) error {

	return nil
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
