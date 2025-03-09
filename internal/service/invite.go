package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/yourusername/eidolon/internal/models"
	"github.com/yourusername/eidolon/internal/repository"
)

// InviteService предоставляет методы для управления инвайт-кодами
type InviteService struct {
	repo repository.Repository
}

// NewInviteService создает новый сервис управления инвайт-кодами
func NewInviteService(repo repository.Repository) *InviteService {
	return &InviteService{
		repo: repo,
	}
}

// GenerateInviteCode генерирует новый инвайт-код для пользователя
func (s *InviteService) GenerateInviteCode(ctx context.Context, userID int64) (*models.InviteCode, error) {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Проверяем, что пользователь имеет право создавать инвайт-коды
	userLimits := user.GetRoleLimits()
	if userLimits.MaxInvites == 0 {
		return nil, fmt.Errorf("user does not have permission to create invite codes")
	}

	// Проверяем, не превышен ли лимит инвайт-кодов
	if userLimits.MaxInvites > 0 {
		activeInvites, err := s.repo.Invite().CountActiveByCreator(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to count active invites: %w", err)
		}

		if activeInvites >= userLimits.MaxInvites {
			return nil, fmt.Errorf("invite code limit reached")
		}
	}

	// Генерируем уникальный инвайт-код
	code, err := generateRandomCode(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %w", err)
	}

	// Создаем инвайт-код
	invite := &models.InviteCode{
		Code:      code,
		CreatedBy: userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().AddDate(0, 0, 7), // Действителен 7 дней
		Expired:   false,
	}

	// Сохраняем инвайт-код в базе данных
	err = s.repo.Invite().Create(ctx, invite)
	if err != nil {
		return nil, fmt.Errorf("failed to create invite code: %w", err)
	}

	return invite, nil
}

// UseInviteCode использует инвайт-код для создания нового пользователя
func (s *InviteService) UseInviteCode(ctx context.Context, code string, newUser *models.User) error {
	// Получаем инвайт-код из базы данных
	invite, err := s.repo.Invite().GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("invalid invite code: %w", err)
	}

	// Проверяем, что инвайт-код действителен
	if !invite.IsValid() {
		return fmt.Errorf("invite code is expired or already used")
	}

	// Получаем создателя инвайт-кода
	inviter, err := s.repo.User().GetByID(ctx, invite.CreatedBy)
	if err != nil {
		return fmt.Errorf("failed to get inviter: %w", err)
	}

	// Определяем роль нового пользователя
	// Если инвайтер имеет роль admin, то новый пользователь получает роль user
	// Иначе новый пользователь получает роль vassal
	if inviter.Role == models.RoleAdmin {
		newUser.Role = models.RoleUser
	} else {
		newUser.Role = models.RoleVassal
	}

	// Устанавливаем ссылку на инвайтера
	newUser.InvitedBy = inviter.ID

	// Устанавливаем дату создания
	newUser.CreatedAt = time.Now()

	// Создаем пользователя
	err = s.repo.User().Create(ctx, newUser)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Отмечаем инвайт-код как использованный
	invite.UsedBy = newUser.ID
	invite.UsedAt = time.Now()
	invite.Expired = true

	err = s.repo.Invite().Update(ctx, invite)
	if err != nil {
		return fmt.Errorf("failed to update invite code: %w", err)
	}

	return nil
}

// GetInviteCodes возвращает список инвайт-кодов, созданных пользователем
func (s *InviteService) GetInviteCodes(ctx context.Context, userID int64) ([]*models.InviteCode, error) {
	return s.repo.Invite().ListByCreator(ctx, userID)
}

// DeleteInviteCode удаляет инвайт-код
func (s *InviteService) DeleteInviteCode(ctx context.Context, inviteID int64, userID int64) error {
	// Получаем инвайт-код
	invite, err := s.repo.Invite().GetByID(ctx, inviteID)
	if err != nil {
		return fmt.Errorf("invite code not found: %w", err)
	}

	// Проверяем, что пользователь является создателем инвайт-кода или админом
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if invite.CreatedBy != userID && user.Role != models.RoleAdmin {
		return fmt.Errorf("you don't have permission to delete this invite code")
	}

	return s.repo.Invite().Delete(ctx, inviteID)
}

// GetInviteTree возвращает "дерево" инвайтов пользователя
func (s *InviteService) GetInviteTree(ctx context.Context, userID int64) (map[int64][]*models.User, error) {
	// Получаем пользователя
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Проверяем, что пользователь имеет право просматривать дерево инвайтов
	userLimits := user.GetRoleLimits()
	if !userLimits.CanViewInviteTree {
		return nil, fmt.Errorf("user does not have permission to view invite tree")
	}

	// Получаем пользователей, приглашенных текущим пользователем
	invitedUsers, err := s.repo.User().GetInvitedUsers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invited users: %w", err)
	}

	// Если пользователь не admin, то возвращаем только первый уровень
	if user.Role != models.RoleAdmin {
		tree := make(map[int64][]*models.User)
		tree[userID] = invitedUsers
		return tree, nil
	}

	// Для админа строим полное дерево
	tree := make(map[int64][]*models.User)
	tree[userID] = invitedUsers

	// Рекурсивно получаем приглашенных пользователей
	for _, invitedUser := range invitedUsers {
		subTree, err := s.buildInviteTree(ctx, invitedUser.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to build invite tree: %w", err)
		}

		// Объединяем поддерево с основным
		for id, users := range subTree {
			tree[id] = users
		}
	}

	return tree, nil
}

// buildInviteTree рекурсивно строит дерево инвайтов
func (s *InviteService) buildInviteTree(ctx context.Context, userID int64) (map[int64][]*models.User, error) {
	invitedUsers, err := s.repo.User().GetInvitedUsers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invited users: %w", err)
	}

	tree := make(map[int64][]*models.User)
	tree[userID] = invitedUsers

	// Базовый случай: нет приглашенных пользователей
	if len(invitedUsers) == 0 {
		return tree, nil
	}

	// Рекурсивный случай: есть приглашенные пользователи
	for _, invitedUser := range invitedUsers {
		subTree, err := s.buildInviteTree(ctx, invitedUser.ID)
		if err != nil {
			return nil, err
		}

		// Объединяем поддерево с текущим
		for id, users := range subTree {
			tree[id] = users
		}
	}

	return tree, nil
}

// generateRandomCode генерирует случайный код заданной длины
func generateRandomCode(length int) (string, error) {
	// Вычисляем, сколько байт нам нужно для получения заданной длины base64-строки
	// Base64 кодирует 3 байта в 4 символа, поэтому:
	byteLength := length * 3 / 4

	// Генерируем случайные байты
	randomBytes := make([]byte, byteLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Кодируем в base64
	code := base64.URLEncoding.EncodeToString(randomBytes)

	// Обрезаем до нужной длины
	if len(code) > length {
		code = code[:length]
	}

	return code, nil
}
