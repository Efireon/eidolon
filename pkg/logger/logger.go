package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Setup настраивает логгер с указанным уровнем и директорией для логов
func Setup(level string, logDir string) (*logrus.Logger, error) {
	logger := logrus.New()

	// Устанавливаем формат логов
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Устанавливаем уровень логирования
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Если директория для логов не указана, выводим логи только в stdout
	if logDir == "" {
		return logger, nil
	}

	// Создаем директорию для логов, если она не существует
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return logger, err
	}

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(
		filepath.Join(logDir, "eidolon.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return logger, err
	}

	// Дублируем логи в файл и в стандартный вывод
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger.SetOutput(multiWriter)

	return logger, nil
}
