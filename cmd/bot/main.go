package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eidolon/internal/bot"
	"eidolon/internal/config"
	"eidolon/internal/repository"
	"eidolon/internal/service"
	"eidolon/pkg/logger"
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
	log, err := logger.Setup(cfg.LogLevel, "logs")
	if err != nil {
		fmt.Printf("Failed to set up logger: %v\n", err)
		os.Exit(1)
	}

	log.Info("Starting Eidolon Telegram Bot")

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Подключаемся к базе данных
	repo, err := repository.NewPostgresRepository(cfg.Database.ConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	// Создаем сервисы
	authService := service.NewAuthService(repo, cfg.JWT.Secret, time.Duration(cfg.JWT.ExpiryMinutes)*time.Minute)
	inviteService := service.NewInviteService(repo)

	// Создаем Telegram бота
	telegramBot, err := bot.NewTelegramBot(
		cfg.Telegram.Token,
		authService,
		inviteService,
		repo,
		log,
		cfg.Telegram.AdminIDs,
	)

	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Запускаем бота в отдельной горутине
	go func() {
		if err := telegramBot.Start(ctx); err != nil {
			log.Fatalf("Failed to start Telegram bot: %v", err)
		}
	}()

	log.Info("Telegram bot started")

	// Ожидаем сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Received shutdown signal")

	// Отменяем контекст, чтобы остановить бота
	cancel()

	// Ждем немного для корректного завершения
	time.Sleep(1 * time.Second)

	log.Info("Telegram bot stopped")
}
