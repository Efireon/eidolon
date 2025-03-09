package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"eidolon/internal/bot"
	"eidolon/internal/config"
	"eidolon/internal/repository"
	"eidolon/internal/service"
	"eidolon/internal/vpn"

	"github.com/sirupsen/logrus"
)

var (
	configPath string
)

func init() {
	flag.StringVar(&configPath, "config", "configs/config.yaml", "Path to configuration file")
}

func main() {
	flag.Parse()

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Настраиваем логгер
	logger := setupLogger(cfg.LogLevel)
	logger.Info("Starting Eidolon VPN service")

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Подключаемся к базе данных
	repo, err := repository.NewPostgresRepository(cfg.Database.ConnectionString)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	// Создаем менеджер сертификатов
	certManager, err := vpn.NewCertificateManager(cfg.VPN.CertDirectory)
	if err != nil {
		logger.Fatalf("Failed to create certificate manager: %v", err)
	}

	// Загружаем или создаем CA сертификат
	err = certManager.LoadOrCreateCA(vpn.CertOptions{
		CommonName:   cfg.VPN.CACommonName,
		Organization: cfg.VPN.Organization,
		Country:      cfg.VPN.Country,
		ValidForDays: 3650, // 10 лет
	})
	if err != nil {
		logger.Fatalf("Failed to load or create CA certificate: %v", err)
	}

	// Загружаем или создаем сертификат сервера
	err = certManager.LoadOrCreateServerCert(vpn.CertOptions{
		CommonName:   cfg.VPN.ServerCommonName,
		Organization: cfg.VPN.Organization,
		Country:      cfg.VPN.Country,
		ValidForDays: 3650, // 10 лет
	})
	if err != nil {
		logger.Fatalf("Failed to load or create server certificate: %v", err)
	}

	// Создаем VPN сервер
	vpnServer := vpn.NewOpenConnectServer(
		vpn.WithListenIP(cfg.VPN.ListenIP),
		vpn.WithListenPort(cfg.VPN.ListenPort),
		vpn.WithCertificate(
			certManager.GetServerCertFilePath(),
			certManager.GetServerKeyFilePath(),
		),
		vpn.WithCA(certManager.GetCAFilePath()),
		vpn.WithLogger(logger),
	)

	// Создаем сервисы
	authService := service.NewAuthService(repo, cfg.JWT.Secret, time.Duration(cfg.JWT.ExpiryMinutes)*time.Minute)
	inviteService := service.NewInviteService(repo)
	vpnService := service.NewVPNService(repo, vpnServer, certManager, logger, cfg.VPN.DefaultRoutes, cfg.VPN.DefaultASNRoutes)

	// Создаем Telegram бота
	telegramBot, err := bot.NewTelegramBot(
		cfg.Telegram.Token,
		authService,
		inviteService,
		vpnService,
		repo, // Добавлен репозиторий как аргумент
		logger,
		cfg.Telegram.AdminIDs,
	)
	if err != nil {
		logger.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Запускаем VPN сервер
	logger.Info("Starting VPN server")
	if err := vpnService.Start(ctx); err != nil {
		logger.Fatalf("Failed to start VPN server: %v", err)
	}

	// Запускаем Telegram бота в отдельной горутине
	go func() {
		logger.Info("Starting Telegram bot")
		if err := telegramBot.Start(ctx); err != nil {
			logger.Fatalf("Failed to start Telegram bot: %v", err)
		}
	}()

	// Ожидаем сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Received shutdown signal")

	// Создаем контекст с таймаутом для корректного завершения
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Останавливаем VPN сервер
	logger.Info("Stopping VPN server")
	if err := vpnService.Stop(); err != nil {
		logger.Errorf("Failed to stop VPN server: %v", err)
	}

	// Отменяем контекст, чтобы остановить бота
	cancel()

	// Ожидаем завершения всех компонентов
	<-shutdownCtx.Done()
	logger.Info("Eidolon VPN service stopped")
}

// setupLogger настраивает логгер
func setupLogger(level string) *logrus.Logger {
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

	// Создаем директорию для логов, если она не существует
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		logger.Warnf("Failed to create log directory: %v", err)
	} else {
		// Открываем файл для записи логов
		logFile, err := os.OpenFile(
			filepath.Join(logDir, "eidolon.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644,
		)
		if err != nil {
			logger.Warnf("Failed to open log file: %v", err)
		} else {
			// Дублируем логи в файл и в стандартный вывод
			logger.SetOutput(logFile)
		}
	}

	return logger
}
