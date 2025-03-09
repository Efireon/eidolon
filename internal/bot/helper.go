package bot

import (
	"context"

	"eidolon/internal/models"
	"eidolon/pkg/utils"
)

// updateUserRole обновляет роль пользователя в базе данных
func (b *TelegramBot) updateUserRole(ctx context.Context, user *models.User) error {
	return b.repo.User().Update(ctx, user)
}

// formatTraffic форматирует количество байт в человекочитаемый формат
func formatTraffic(bytes int64) string {
	return utils.FormatTraffic(bytes)
}
