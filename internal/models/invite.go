package models

import (
	"time"
)

// InviteCode представляет инвайт-код для добавления пользователей
type InviteCode struct {
	ID        int64     `json:"id" db:"id"`
	Code      string    `json:"code" db:"code"`
	CreatedBy int64     `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UsedBy    int64     `json:"used_by,omitempty" db:"used_by"`
	UsedAt    time.Time `json:"used_at,omitempty" db:"used_at"`
	Expired   bool      `json:"expired" db:"expired"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}

// IsValid проверяет, действителен ли инвайт-код
func (i *InviteCode) IsValid() bool {
	// Код действителен, если он не истек, не использован и не просрочен
	return !i.Expired && i.UsedBy == 0 && time.Now().Before(i.ExpiresAt)
}

// GetTimeRangeFromPeriod возвращает временной диапазон на основе указанного периода
func (u *User) GetTimeRangeFromPeriod(period string) (time.Time, time.Time) {
	now := time.Now()
	var from time.Time

	switch period {
	case "day":
		from = now.AddDate(0, 0, -1)
	case "week":
		from = now.AddDate(0, 0, -7)
	case "month":
		from = now.AddDate(0, -1, 0)
	case "year":
		from = now.AddDate(-1, 0, 0)
	default:
		from = now.AddDate(0, 0, -30) // по умолчанию 30 дней
	}

	return from, now
}
