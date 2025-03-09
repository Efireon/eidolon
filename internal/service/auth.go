package service

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"eidolon/internal/models"
	"eidolon/internal/repository"

	"github.com/golang-jwt/jwt/v4"
)

// Ошибки сервиса аутентификации
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
)

// Claims представляет JWT-токен с пользовательскими данными
type Claims struct {
	UserID   int64           `json:"user_id"`
	Username string          `json:"username"`
	Role     models.RoleType `json:"role"`
	jwt.RegisteredClaims
}

// AuthService предоставляет методы для аутентификации и авторизации
type AuthService struct {
	repo      repository.Repository
	jwtSecret []byte
	tokenTTL  time.Duration
}

// NewAuthService создает новый сервис аутентификации
func NewAuthService(repo repository.Repository, jwtSecret string, tokenTTL time.Duration) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  tokenTTL,
	}
}

// RegisterUserWithTelegram регистрирует нового пользователя через Telegram
func (s *AuthService) RegisterUserWithTelegram(ctx context.Context, telegramID int64, username string) (*models.User, error) {
	// Проверяем, существует ли уже пользователь с таким Telegram ID
	existingUser, err := s.repo.User().GetByTelegramID(ctx, telegramID)
	if err == nil {
		// Пользователь уже существует
		existingUser.LastLoginAt = time.Now()
		if err := s.repo.User().Update(ctx, existingUser); err != nil {
			return nil, fmt.Errorf("failed to update last login time: %w", err)
		}
		return existingUser, nil
	}

	// Создаем нового пользователя
	user := &models.User{
		Username:    username,
		TelegramID:  telegramID,
		Role:        models.RoleVassal, // По умолчанию роль vassal
		CreatedAt:   time.Now(),
		LastLoginAt: time.Now(),
	}

	// Пока нет возможности использовать инвайт-код, пользователь будет иметь роль vassal
	// и должен быть активирован администратором

	if err := s.repo.User().Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// AuthenticateWithTelegram выполняет аутентификацию пользователя через Telegram
func (s *AuthService) AuthenticateWithTelegram(ctx context.Context, telegramID int64) (*models.User, error) {
	user, err := s.repo.User().GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Обновляем время последнего входа
	user.LastLoginAt = time.Now()
	if err := s.repo.User().Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update last login time: %w", err)
	}

	return user, nil
}

// AuthenticateWithCertificate выполняет аутентификацию пользователя по сертификату
func (s *AuthService) AuthenticateWithCertificate(ctx context.Context, certPEM string) (*models.User, error) {
	// Парсим PEM-блок сертификата
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Парсим сертификат
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Ищем пользователя по CommonName из сертификата
	user, err := s.repo.User().GetByUsername(ctx, cert.Subject.CommonName)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Проверяем, что сертификат пользователя совпадает с предоставленным
	userCert, err := user.ParseCertificate()
	if err != nil {
		return nil, fmt.Errorf("failed to parse user certificate: %w", err)
	}

	// Сравниваем серийные номера сертификатов
	if cert.SerialNumber.Cmp(userCert.SerialNumber) != 0 {
		return nil, ErrInvalidCredentials
	}

	// Обновляем время последнего входа
	user.LastLoginAt = time.Now()
	if err := s.repo.User().Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update last login time: %w", err)
	}

	return user, nil
}

// GenerateToken генерирует JWT-токен для пользователя
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	// Создаем claims для JWT-токена
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	// Создаем токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken проверяет и парсит JWT-токен
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	// Парсим токен
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return s.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Извлекаем claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// CheckUserPermission проверяет, имеет ли пользователь указанную роль или выше
func (s *AuthService) CheckUserPermission(userRole models.RoleType, requiredRole models.RoleType) bool {
	// Проверяем роль пользователя
	switch requiredRole {
	case models.RoleAdmin:
		return userRole == models.RoleAdmin
	case models.RoleUser:
		return userRole == models.RoleAdmin || userRole == models.RoleUser
	case models.RoleVassal:
		return userRole == models.RoleAdmin || userRole == models.RoleUser || userRole == models.RoleVassal
	default:
		return false
	}
}

// GetUserByID получает пользователя по его ID
func (s *AuthService) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return user, nil
}
