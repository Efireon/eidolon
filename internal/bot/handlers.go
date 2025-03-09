package bot

import (
	"context"
	"fmt"
	"time"

	"eidolon/internal/models"
	"eidolon/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleStatusCommand обрабатывает команду /status
func (b *TelegramBot) handleStatusCommand(ctx context.Context, chatID int64, user *models.User) {
	// Получаем активные подключения
	activeConnections, err := b.vpnService.GetActiveConnections(ctx)
	if err != nil {
		b.logger.Errorf("Failed to get active connections: %v", err)
		b.sendMessage(chatID, "Ошибка при получении статуса VPN.")
		return
	}

	// Формируем сообщение о статусе
	statusMsg := "Статус VPN:\n\n"
	statusMsg += fmt.Sprintf("Активных подключений: %d\n", len(activeConnections))

	// Показываем активные подключения (только для админов)
	if user.Role == models.RoleAdmin && len(activeConnections) > 0 {
		statusMsg += "\nАктивные пользователи:\n"
		for userID, username := range activeConnections {
			statusMsg += fmt.Sprintf("- %s (ID: %d)\n", username, userID)
		}
	}

	// Показываем информацию о пользователе
	userInfo := fmt.Sprintf("\nВаша информация:\n"+
		"Имя пользователя: %s\n"+
		"Роль: %s\n", user.Username, user.Role)

	statusMsg += userInfo

	// Показываем статистику трафика
	totalTraffic, err := b.vpnService.GetTotalUserTraffic(ctx, user.ID)
	if err != nil {
		b.logger.Warnf("Failed to get user traffic: %v", err)
	} else {
		// Конвертируем байты в более читаемый формат
		traffic := utils.FormatTraffic(totalTraffic)
		statusMsg += fmt.Sprintf("Использовано трафика: %s\n", traffic)
	}

	b.sendMessage(chatID, statusMsg)
}

// handleRoutesCommand обрабатывает команду /routes
func (b *TelegramBot) handleRoutesCommand(ctx context.Context, chatID int64, user *models.User) {
	// Проверяем, что пользователь имеет право просматривать маршруты
	userLimits := user.GetRoleLimits()
	if !userLimits.CanAddRoutes {
		b.sendMessage(chatID, "У вас нет прав на просмотр и управление маршрутами.")
		return
	}

	// Получаем маршруты пользователя
	routes, err := b.vpnService.GetUserRoutes(ctx, user.ID)
	if err != nil {
		b.logger.Errorf("Failed to get user routes: %v", err)
		b.sendMessage(chatID, "Ошибка при получении списка маршрутов.")
		return
	}

	// Формируем сообщение со списком маршрутов
	msg := "Ваши маршруты:\n\n"

	if len(routes) == 0 {
		msg += "У вас пока нет настроенных маршрутов.\n"
		msg += "Используйте /addroute [сеть CIDR] для добавления маршрута."
	} else {
		for i, route := range routes {
			msg += fmt.Sprintf("%d. %s\n   Тип: %s\n   Описание: %s\n\n",
				i+1, route.Network, route.Type, route.Description)
		}
	}

	// Создаем клавиатуру для управления маршрутами
	var keyboard [][]tgbotapi.InlineKeyboardButton

	// Добавляем кнопку для добавления маршрута
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Добавить маршрут", "route:add"),
	})

	// Если у пользователя есть маршруты, добавляем кнопки для управления ими
	if len(routes) > 0 {
		// Добавляем кнопку для удаления маршрутов
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Удалить маршрут", "route:delete"),
		})
	}

	message := tgbotapi.NewMessage(chatID, msg)
	if len(keyboard) > 0 {
		message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	}

	_, err = b.bot.Send(message)
	if err != nil {
		b.logger.Errorf("Failed to send routes message: %v", err)
	}
}

// handleAddRouteCommand обрабатывает команду /addroute
func (b *TelegramBot) handleAddRouteCommand(ctx context.Context, chatID int64, user *models.User, args string) {
	// Проверяем, что пользователь имеет право добавлять маршруты
	userLimits := user.GetRoleLimits()
	if !userLimits.CanAddRoutes {
		b.sendMessage(chatID, "У вас нет прав на добавление маршрутов.")
		return
	}

	if args == "" {
		b.sendMessage(chatID, "Укажите сеть в формате CIDR. Пример: /addroute 192.168.0.0/24")
		return
	}

	// Валидируем CIDR
	network, err := utils.ValidateCIDR(args)
	if err != nil {
		b.sendMessage(chatID, fmt.Sprintf("Неверный формат CIDR: %v", err))
		return
	}

	// Создаем новый маршрут
	route := &models.Route{
		Network:     network,
		Description: "Добавлен через Telegram",
		Type:        models.RouteTypeCustom,
		CreatedBy:   user.ID,
		CreatedAt:   time.Now(),
	}

	// Добавляем маршрут
	err = b.vpnService.CreateRoute(ctx, route)
	if err != nil {
		b.logger.Errorf("Failed to create route: %v", err)
		b.sendMessage(chatID, fmt.Sprintf("Ошибка при добавлении маршрута: %v", err))
		return
	}

	// Добавляем маршрут для пользователя
	err = b.vpnService.AddUserRoute(ctx, user.ID, route.ID)
	if err != nil {
		b.logger.Errorf("Failed to add user route: %v", err)
		b.sendMessage(chatID, "Маршрут создан, но возникла ошибка при добавлении его для вас.")
		return
	}

	b.sendMessage(chatID, fmt.Sprintf("Маршрут %s успешно добавлен!", network))
}

// handleTrafficCommand обрабатывает команду /traffic
func (b *TelegramBot) handleTrafficCommand(ctx context.Context, chatID int64, user *models.User) {
	// Получаем статистику трафика пользователя
	// За последние 30 дней
	now := time.Now()
	from := now.AddDate(0, 0, -30).Unix()
	to := now.Unix()

	trafficStats, err := b.vpnService.GetUserTraffic(ctx, user.ID, from, to)
	if err != nil {
		b.logger.Errorf("Failed to get user traffic: %v", err)
		b.sendMessage(chatID, "Ошибка при получении статистики трафика.")
		return
	}

	// Формируем сообщение со статистикой трафика
	msg := "Статистика использования трафика:\n\n"

	if len(trafficStats) == 0 {
		msg += "У вас пока нет данных о трафике."
	} else {
		// Расчет общего трафика
		var totalBytes int64
		for _, stat := range trafficStats {
			totalBytes += stat.Bytes
		}

		// Форматируем общий трафик
		totalTraffic := utils.FormatTraffic(totalBytes)
		msg += fmt.Sprintf("Общий трафик за 30 дней: %s\n\n", totalTraffic)

		// Получаем суточную статистику
		dailyStats := aggregateDailyTraffic(trafficStats)

		// Выводим статистику по дням (последние 7 дней)
		days := 0
		for date, bytes := range dailyStats {
			if days >= 7 {
				break
			}
			traffic := utils.FormatTraffic(bytes)
			msg += fmt.Sprintf("%s: %s\n", date, traffic)
			days++
		}
	}

	// Создаем клавиатуру для выбора периода
	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("День", "traffic:day"),
			tgbotapi.NewInlineKeyboardButtonData("Неделя", "traffic:week"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Месяц", "traffic:month"),
			tgbotapi.NewInlineKeyboardButtonData("Год", "traffic:year"),
		},
	}

	message := tgbotapi.NewMessage(chatID, msg)
	message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	_, err = b.bot.Send(message)
	if err != nil {
		b.logger.Errorf("Failed to send traffic message: %v", err)
	}
}

// handleGenerateCommand обрабатывает команду /generate
func (b *TelegramBot) handleGenerateCommand(ctx context.Context, chatID int64, user *models.User) {
	// Проверяем, что пользователь имеет право генерировать инвайт-коды
	userLimits := user.GetRoleLimits()
	if userLimits.MaxInvites == 0 {
		b.sendMessage(chatID, "У вас нет прав на генерацию инвайт-кодов.")
		return
	}

	// Если есть лимит на инвайт-коды, проверяем его
	if userLimits.MaxInvites > 0 {
		activeInvites, err := b.repo.Invite().CountActiveByCreator(ctx, user.ID)
		if err != nil {
			b.logger.Errorf("Failed to count active invites: %v", err)
			b.sendMessage(chatID, "Ошибка при проверке лимита инвайт-кодов.")
			return
		}

		if activeInvites >= userLimits.MaxInvites {
			b.sendMessage(chatID, fmt.Sprintf("Вы достигли лимита активных инвайт-кодов (%d). Удалите неиспользованные коды или дождитесь их использования.", userLimits.MaxInvites))
			return
		}
	}

	// Генерируем инвайт-код
	invite, err := b.inviteService.GenerateInviteCode(ctx, user.ID)
	if err != nil {
		b.logger.Errorf("Failed to generate invite code: %v", err)
		b.sendMessage(chatID, fmt.Sprintf("Ошибка при генерации инвайт-кода: %v", err))
		return
	}

	// Отправляем сообщение с инвайт-кодом
	msg := fmt.Sprintf("Инвайт-код успешно сгенерирован!\n\nКод: `%s`\n\nДействителен до: %s",
		invite.Code, invite.ExpiresAt.Format("02.01.2006 15:04:05"))

	// Создаем сообщение с Markdown форматированием для выделения кода
	message := tgbotapi.NewMessage(chatID, msg)
	message.ParseMode = "Markdown"
	message.ReplyMarkup = b.createInviteKeyboard(invite.ID)

	_, err = b.bot.Send(message)
	if err != nil {
		b.logger.Errorf("Failed to send message: %v", err)
	}
}

// handleInviteCommand обрабатывает команду /invite
func (b *TelegramBot) handleInviteCommand(ctx context.Context, chatID int64, user *models.User, args string) {
	if args == "" {
		b.sendMessage(chatID, "Укажите инвайт-код. Пример: /invite ABC123XYZ")
		return
	}

	// Попытка использовать инвайт-код
	// Создаем временный пользователь с существующими данными
	tempUser := &models.User{
		ID:         user.ID,
		Username:   user.Username,
		TelegramID: user.TelegramID,
	}

	err := b.inviteService.UseInviteCode(ctx, args, tempUser)
	if err != nil {
		b.logger.Errorf("Failed to use invite code: %v", err)
		b.sendMessage(chatID, fmt.Sprintf("Ошибка при активации инвайт-кода: %v", err))
		return
	}

	// Обновляем пользователя с новой ролью
	user.Role = tempUser.Role
	user.InvitedBy = tempUser.InvitedBy

	err = b.updateUserRole(ctx, user)
	if err != nil {
		b.logger.Errorf("Failed to update user role: %v", err)
		b.sendMessage(chatID, "Ошибка при обновлении роли пользователя.")
		return
	}

	// Генерируем сертификат для пользователя
	_, err = b.vpnService.CreateUserCertificate(ctx, user)
	if err != nil {
		b.logger.Errorf("Failed to create user certificate: %v", err)
		b.sendMessage(chatID, "Инвайт-код активирован, но возникла ошибка при создании сертификата.")
		return
	}

	b.sendMessage(chatID, fmt.Sprintf("Инвайт-код успешно активирован!\nВаша новая роль: %s\n\nИспользуйте /config для получения конфигурации VPN.", user.Role))
}

// handleMyInvitesCommand обрабатывает команду /myinvites
func (b *TelegramBot) handleMyInvitesCommand(ctx context.Context, chatID int64, user *models.User) {
	// Проверяем, что пользователь имеет право просматривать инвайт-коды
	userLimits := user.GetRoleLimits()
	if !userLimits.CanManageInvites {
		b.sendMessage(chatID, "У вас нет прав на просмотр инвайт-кодов.")
		return
	}

	// Получаем список инвайт-кодов пользователя
	invites, err := b.inviteService.GetInviteCodes(ctx, user.ID)
	if err != nil {
		b.logger.Errorf("Failed to get invite codes: %v", err)
		b.sendMessage(chatID, "Ошибка при получении списка инвайт-кодов.")
		return
	}

	// Если у пользователя нет инвайт-кодов
	if len(invites) == 0 {
		b.sendMessage(chatID, "У вас пока нет инвайт-кодов. Используйте /generate для создания нового инвайт-кода.")
		return
	}

	// Формируем сообщение со списком инвайт-кодов
	var activeInvites, usedInvites, expiredInvites []*models.InviteCode

	// Разделяем инвайт-коды по статусу
	for _, invite := range invites {
		if invite.UsedBy > 0 {
			usedInvites = append(usedInvites, invite)
		} else if invite.Expired || time.Now().After(invite.ExpiresAt) {
			expiredInvites = append(expiredInvites, invite)
		} else {
			activeInvites = append(activeInvites, invite)
		}
	}

	msg := "Ваши инвайт-коды:\n\n"

	// Добавляем активные инвайт-коды
	if len(activeInvites) > 0 {
		msg += "Активные:\n"
		for i, invite := range activeInvites {
			msg += fmt.Sprintf("%d. Код: `%s`\n   Истекает: %s\n\n",
				i+1, invite.Code, invite.ExpiresAt.Format("02.01.2006 15:04:05"))
		}
	}

	// Добавляем использованные инвайт-коды
	if len(usedInvites) > 0 {
		msg += "Использованные:\n"
		for i, invite := range usedInvites {
			msg += fmt.Sprintf("%d. Код: %s\n   Использован: %s\n\n",
				i+1, invite.Code, invite.UsedAt.Format("02.01.2006 15:04:05"))
		}
	}

	// Добавляем истекшие инвайт-коды
	if len(expiredInvites) > 0 {
		msg += "Истекшие:\n"
		for i, invite := range expiredInvites {
			msg += fmt.Sprintf("%d. Код: %s\n   Истек: %s\n\n",
				i+1, invite.Code, invite.ExpiresAt.Format("02.01.2006 15:04:05"))
		}
	}

	// Отправляем сообщение с Markdown форматированием
	message := tgbotapi.NewMessage(chatID, msg)
	message.ParseMode = "Markdown"

	// Если есть активные инвайт-коды, добавляем кнопку для генерации нового
	if len(activeInvites) > 0 && (userLimits.MaxInvites <= 0 || len(activeInvites) < userLimits.MaxInvites) {
		keyboard := [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("Создать новый", "invite:generate"),
			},
		}
		message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	}

	_, err = b.bot.Send(message)
	if err != nil {
		b.logger.Errorf("Failed to send message: %v", err)
		b.sendMessage(chatID, "Ошибка при отправке списка инвайт-кодов.")
	}
}

// handleDisconnectCommand обрабатывает команду /disconnect
func (b *TelegramBot) handleDisconnectCommand(ctx context.Context, chatID int64, user *models.User, args string) {
	// Проверяем, что пользователь имеет права администратора
	if user.Role != models.RoleAdmin {
		b.sendMessage(chatID, "У вас нет прав на отключение пользователей.")
		return
	}

	if args == "" {
		b.sendMessage(chatID, "Укажите имя пользователя для отключения. Пример: /disconnect username")
		return
	}

	// Находим пользователя по имени
	targetUser, err := b.repo.User().GetByUsername(ctx, args)
	if err != nil {
		b.logger.Errorf("Failed to find user %s: %v", args, err)
		b.sendMessage(chatID, fmt.Sprintf("Пользователь %s не найден.", args))
		return
	}

	// Отключаем пользователя
	err = b.vpnService.DisconnectUser(ctx, targetUser.ID)
	if err != nil {
		b.logger.Errorf("Failed to disconnect user %s: %v", args, err)
		b.sendMessage(chatID, fmt.Sprintf("Ошибка при отключении пользователя %s: %v", args, err))
		return
	}

	b.sendMessage(chatID, fmt.Sprintf("Пользователь %s успешно отключен от VPN.", args))
}

// handleUsersCommand обрабатывает команду /users
func (b *TelegramBot) handleUsersCommand(ctx context.Context, chatID int64, user *models.User, args string) {
	// Проверяем, что пользователь имеет права администратора
	if user.Role != models.RoleAdmin {
		b.sendMessage(chatID, "У вас нет прав на управление пользователями.")
		return
	}

	// Получаем список пользователей
	users, err := b.repo.User().List(ctx, 0, 100) // Ограничиваем 100 пользователями
	if err != nil {
		b.logger.Errorf("Failed to get users list: %v", err)
		b.sendMessage(chatID, "Ошибка при получении списка пользователей.")
		return
	}

	// Формируем сообщение со списком пользователей
	msg := "Список пользователей:\n\n"

	if len(users) == 0 {
		msg += "Пользователи не найдены."
	} else {
		for i, u := range users {
			lastLogin := "Никогда"
			if !u.LastLoginAt.IsZero() {
				lastLogin = u.LastLoginAt.Format("02.01.2006 15:04:05")
			}

			msg += fmt.Sprintf("%d. %s (ID: %d)\n   Роль: %s\n   Последний вход: %s\n\n",
				i+1, u.Username, u.ID, u.Role, lastLogin)
		}
	}

	// Создаем клавиатуру с кнопками управления пользователями
	var keyboard [][]tgbotapi.InlineKeyboardButton

	// Добавляем кнопки, если есть пользователи
	if len(users) > 0 {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Повысить", "user:promote"),
			tgbotapi.NewInlineKeyboardButtonData("Понизить", "user:demote"),
		})
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Отключить", "user:disconnect"),
			tgbotapi.NewInlineKeyboardButtonData("Удалить", "user:delete"),
		})
	}

	message := tgbotapi.NewMessage(chatID, msg)
	if len(keyboard) > 0 {
		message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	}

	_, err = b.bot.Send(message)
	if err != nil {
		b.logger.Errorf("Failed to send users list: %v", err)
	}
}

// handleConfigCommand обрабатывает команду /config
func (b *TelegramBot) handleConfigCommand(ctx context.Context, chatID int64, user *models.User) {
	// Проверяем, что у пользователя есть сертификат
	if user.Certificate == "" {
		b.sendMessage(chatID, "У вас нет настроенного сертификата. Сначала активируйте инвайт-код с помощью команды /invite.")
		return
	}

	// Формируем конфигурационный файл для клиента OpenConnect
	config := fmt.Sprintf(`# Eidolon VPN конфигурация OpenConnect
# Имя: %s
# Создано: %s

server=vpn.example.com
port=443
protocol=tcp
user=%s
authgroup=Eidolon

-----BEGIN CERTIFICATE-----
%s
-----END CERTIFICATE-----
`, user.Username, time.Now().Format("02.01.2006 15:04:05"), user.Username, user.Certificate)

	// Создаем документ для отправки
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
		Name:  fmt.Sprintf("eidolon_config_%s.txt", user.Username),
		Bytes: []byte(config),
	})

	// Добавляем описание
	doc.Caption = "Конфигурация для OpenConnect VPN клиента"

	// Отправляем файл конфигурации
	_, err := b.bot.Send(doc)
	if err != nil {
		b.logger.Errorf("Failed to send config file: %v", err)
		b.sendMessage(chatID, "Ошибка при отправке файла конфигурации.")
	}
}

// handleTrafficCallback обрабатывает callback для действий с трафиком
func (b *TelegramBot) handleTrafficCallback(ctx context.Context, query *tgbotapi.CallbackQuery, user *models.User, period string) {
	// Получаем временной диапазон
	from, to := utils.GetTimeRangeFromPeriod(period)

	// Получаем статистику трафика за указанный период
	trafficStats, err := b.vpnService.GetUserTraffic(ctx, user.ID, from.Unix(), to.Unix())
	if err != nil {
		b.logger.Errorf("Failed to get user traffic: %v", err)
		b.sendCallbackResponse(query.ID, "Ошибка при получении статистики трафика.")
		return
	}

	// Формируем сообщение со статистикой
	var msg string

	switch period {
	case "day":
		msg = "Статистика трафика за день:\n\n"
	case "week":
		msg = "Статистика трафика за неделю:\n\n"
	case "month":
		msg = "Статистика трафика за месяц:\n\n"
	case "year":
		msg = "Статистика трафика за год:\n\n"
	default:
		msg = "Статистика трафика:\n\n"
	}

	if len(trafficStats) == 0 {
		msg += "Нет данных о трафике за указанный период."
	} else {
		// Расчет общего трафика
		var totalBytes int64
		for _, stat := range trafficStats {
			totalBytes += stat.Bytes
		}

		// Форматируем общий трафик
		totalTraffic := utils.FormatTraffic(totalBytes)
		msg += fmt.Sprintf("Общий трафик: %s\n\n", totalTraffic)

		// Получаем суточную статистику
		dailyStats := aggregateDailyTraffic(trafficStats)

		// Выводим статистику по дням
		for date, bytes := range dailyStats {
			traffic := utils.FormatTraffic(bytes)
			msg += fmt.Sprintf("%s: %s\n", date, traffic)
		}
	}

	// Отправляем ответное сообщение
	b.sendCallbackResponse(query.ID, "Статистика обновлена")

	// Обновляем сообщение с новой статистикой
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, msg)

	// Сохраняем клавиатуру
	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("День", "traffic:day"),
			tgbotapi.NewInlineKeyboardButtonData("Неделя", "traffic:week"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Месяц", "traffic:month"),
			tgbotapi.NewInlineKeyboardButtonData("Год", "traffic:year"),
		},
	}

	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	_, err = b.bot.Send(edit)
	if err != nil {
		b.logger.Errorf("Failed to update traffic message: %v", err)
	}
}

// aggregateDailyTraffic агрегирует статистику трафика по дням
func aggregateDailyTraffic(stats []*models.UserTraffic) map[string]int64 {
	daily := make(map[string]int64)

	for _, stat := range stats {
		date := stat.Timestamp.Format("02.01.2006")
		daily[date] += stat.Bytes
	}

	return daily
}
