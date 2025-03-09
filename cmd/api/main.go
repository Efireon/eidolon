package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eidolon/internal/api"
	"eidolon/internal/config"
	"eidolon/internal/repository"
	"eidolon/internal/service"
	"eidolon/internal/vpn"
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

	log.Info("Starting Eidolon API")

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Подключаемся к базе данных
	repo, err := repository.NewPostgresRepository(cfg.Database.ConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	// Создаем менеджер сертификатов
	certManager, err := vpn.NewCertificateManager(cfg.VPN.CertDirectory)
	if err != nil {
		log.Fatalf("Failed to create certificate manager: %v", err)
	}

	// Загружаем или создаем CA сертификат
	err = certManager.LoadOrCreateCA(vpn.CertOptions{
		CommonName:   cfg.VPN.CACommonName,
		Organization: cfg.VPN.Organization,
		Country:      cfg.VPN.Country,
		ValidForDays: 3650, // 10 лет
	})
	if err != nil {
		log.Fatalf("Failed to load or create CA certificate: %v", err)
	}

	// Загружаем или создаем сертификат сервера
	err = certManager.LoadOrCreateServerCert(vpn.CertOptions{
		CommonName:   cfg.VPN.ServerCommonName,
		Organization: cfg.VPN.Organization,
		Country:      cfg.VPN.Country,
		ValidForDays: 3650, // 10 лет
	})
	if err != nil {
		log.Fatalf("Failed to load or create server certificate: %v", err)
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
		vpn.WithLogger(log),
	)

	// Создаем сервисы
	authService := service.NewAuthService(repo, cfg.JWT.Secret, time.Duration(cfg.JWT.ExpiryMinutes)*time.Minute)
	inviteService := service.NewInviteService(repo)
	vpnService := service.NewVPNService(repo, vpnServer, certManager, log, cfg.VPN.DefaultRoutes, cfg.VPN.DefaultASNRoutes)

	// Создаем и настраиваем API сервер
	serverConfig := api.ServerConfig{
		Addr:            cfg.API.ListenAddr,
		ReadTimeout:     time.Duration(cfg.API.ReadTimeout) * time.Second,
		WriteTimeout:    time.Duration(cfg.API.WriteTimeout) * time.Second,
		ShutdownTimeout: time.Duration(cfg.API.ShutdownTimeout) * time.Second,
	}

	apiServer := api.NewServer(
		serverConfig,
		authService,
		inviteService,
		vpnService,
		log,
	)

	// Запускаем сервер в отдельной горутине
	go func() {
		if err := apiServer.Start(ctx); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()
	log.Infof("API server started on %s", cfg.API.ListenAddr)

	// Ожидаем сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("Received shutdown signal")

	// Останавливаем API сервер
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		log.Errorf("Failed to gracefully stop API server: %v", err)
	}

	log.Info("API server stopped")
}
