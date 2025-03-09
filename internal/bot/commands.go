package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"eidolon/internal/models"
	"eidolon/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleRouteCallback обрабатывает callback для действий с маршрутами
func (b *TelegramBot) handleRouteCallback(ctx context.Context, query *tgbotapi.CallbackQuery, user *models.User, param string) {
	// Проверяем действие
	if param == "add" {
		// Запрашиваем у пользователя ввод CIDR для маршрута
		msg := "Введите сеть в формате CIDR для добавления маршрута. Например: 192.168.0.0/24"
		b.sendMessage(query.Message.Chat.ID, msg)
		b.sendCallbackResponse(query.ID, "Введите CIDR для маршрута")
		return
	}

	if param == "delete" {
		// Получаем маршруты пользователя
		routes, err := b.vpnService.GetUserRoutes(ctx, user.ID)
		if err != nil {
			b.logger.Errorf("Failed to get user routes: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при получении маршрутов")
			return
		}

		// Формируем клавиатуру для выбора маршрута для удаления
		var keyboard [][]tgbotapi.InlineKeyboardButton
		for _, route := range routes {
			// Создаем кнопку для каждого маршрута
			button := tgbotapi.NewInlineKeyboardButtonData(
				route.Network,
				fmt.Sprintf("route:remove:%d", route.ID),
			)
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
		}

		// Добавляем кнопку отмены
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Отмена", "route:cancel"),
		})

		// Отправляем сообщение с клавиатурой
		msg := "Выберите маршрут для удаления:"
		message := tgbotapi.NewMessage(query.Message.Chat.ID, msg)
		message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}

		_, err = b.bot.Send(message)
		if err != nil {
			b.logger.Errorf("Failed to send route delete message: %v", err)
		}

		b.sendCallbackResponse(query.ID, "Выберите маршрут")
		return
	}

	// Если команда содержит remove:ID
	if strings.HasPrefix(param, "remove:") {
		parts := strings.Split(param, ":")
		if len(parts) != 2 {
			b.sendCallbackResponse(query.ID, "Неверный формат команды")
			return
		}

		// Извлекаем ID маршрута
		routeID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid route ID: %v", err)
			b.sendCallbackResponse(query.ID, "Неверный ID маршрута")
			return
		}

		// Удаляем маршрут
		err = b.vpnService.RemoveUserRoute(ctx, user.ID, routeID)
		if err != nil {
			b.logger.Errorf("Failed to remove route: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при удалении маршрута")
			return
		}

		b.sendCallbackResponse(query.ID, "Маршрут удален")
		b.sendMessage(query.Message.Chat.ID, "Маршрут успешно удален.")
		return
	}

	if param == "cancel" {
		b.sendCallbackResponse(query.ID, "Операция отменена")
		return
	}

	b.sendCallbackResponse(query.ID, "Неизвестная команда")
}

// handleGroupCallback обрабатывает callback для действий с группами маршрутов
func (b *TelegramBot) handleGroupCallback(ctx context.Context, query *tgbotapi.CallbackQuery, user *models.User, param string) {
	// Проверяем действие
	if param == "list" {
		// Получаем группы маршрутов пользователя
		groups, err := b.vpnService.GetUserGroups(ctx, user.ID)
		if err != nil {
			b.logger.Errorf("Failed to get user groups: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при получении групп")
			return
		}

		// Формируем сообщение
		msg := "Ваши группы маршрутов:\n\n"
		if len(groups) == 0 {
			msg += "У вас нет групп маршрутов."
		} else {
			for i, group := range groups {
				msg += fmt.Sprintf("%d. %s\n   Описание: %s\n\n", i+1, group.Name, group.Description)
			}
		}

		// Отправляем сообщение
		b.sendMessage(query.Message.Chat.ID, msg)
		b.sendCallbackResponse(query.ID, "Список групп")
		return
	}

	if strings.HasPrefix(param, "routes:") {
		// Извлекаем ID группы
		parts := strings.Split(param, ":")
		if len(parts) != 2 {
			b.sendCallbackResponse(query.ID, "Неверный формат команды")
			return
		}

		groupID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid group ID: %v", err)
			b.sendCallbackResponse(query.ID, "Неверный ID группы")
			return
		}

		// Получаем маршруты в группе
		routes, err := b.vpnService.GetRoutesInGroup(ctx, groupID)
		if err != nil {
			b.logger.Errorf("Failed to get routes in group: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при получении маршрутов")
			return
		}

		// Формируем сообщение
		group, err := b.vpnService.GetRouteGroup(ctx, groupID)
		if err != nil {
			b.logger.Errorf("Failed to get group: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при получении группы")
			return
		}

		msg := fmt.Sprintf("Маршруты в группе '%s':\n\n", group.Name)
		if len(routes) == 0 {
			msg += "В этой группе нет маршрутов."
		} else {
			for i, route := range routes {
				msg += fmt.Sprintf("%d. %s\n   Тип: %s\n   Описание: %s\n\n",
					i+1, route.Network, route.Type, route.Description)
			}
		}

		// Отправляем сообщение
		b.sendMessage(query.Message.Chat.ID, msg)
		b.sendCallbackResponse(query.ID, "Маршруты в группе")
		return
	}

	b.sendCallbackResponse(query.ID, "Неизвестная команда")
}

// handleInviteCallback обрабатывает callback для действий с инвайт-кодами
func (b *TelegramBot) handleInviteCallback(ctx context.Context, query *tgbotapi.CallbackQuery, user *models.User, param string) {
	// Проверяем действие
	if param == "generate" {
		// Генерируем новый инвайт-код
		b.handleGenerateCommand(ctx, query.Message.Chat.ID, user)
		b.sendCallbackResponse(query.ID, "Генерация инвайт-кода")
		return
	}

	if param == "list" {
		// Показываем список инвайт-кодов
		b.handleMyInvitesCommand(ctx, query.Message.Chat.ID, user)
		b.sendCallbackResponse(query.ID, "Список инвайт-кодов")
		return
	}

	// Если команда содержит delete:ID
	if strings.HasPrefix(param, "delete:") {
		parts := strings.Split(param, ":")
		if len(parts) != 2 {
			b.sendCallbackResponse(query.ID, "Неверный формат команды")
			return
		}

		// Извлекаем ID инвайт-кода
		inviteID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid invite ID: %v", err)
			b.sendCallbackResponse(query.ID, "Неверный ID инвайт-кода")
			return
		}

		// Удаляем инвайт-код
		err = b.inviteService.DeleteInviteCode(ctx, inviteID, user.ID)
		if err != nil {
			b.logger.Errorf("Failed to delete invite code: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при удалении инвайт-кода")
			return
		}

		b.sendCallbackResponse(query.ID, "Инвайт-код удален")
		b.sendMessage(query.Message.Chat.ID, "Инвайт-код успешно удален.")
		return
	}

	b.sendCallbackResponse(query.ID, "Неизвестная команда")
}

// handleUserCallback обрабатывает callback для действий с пользователями
func (b *TelegramBot) handleUserCallback(ctx context.Context, query *tgbotapi.CallbackQuery, user *models.User, param string, action string) {
	// Проверяем, что пользователь имеет права администратора
	if user.Role != models.RoleAdmin {
		b.sendCallbackResponse(query.ID, "У вас нет прав на управление пользователями")
		return
	}

	if action == "select" {
		// Получаем список пользователей
		users, err := b.repo.User().List(ctx, 0, 100)
		if err != nil {
			b.logger.Errorf("Failed to get users: %v", err)
			b.sendCallbackResponse(query.ID, "Ошибка при получении пользователей")
			return
		}

		// Формируем клавиатуру для выбора пользователя
		var keyboard [][]tgbotapi.InlineKeyboardButton
		for _, u := range users {
			// Пропускаем текущего пользователя
			if u.ID == user.ID {
				continue
			}

			// Создаем кнопку для пользователя
			button := tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s (%s)", u.Username, u.Role),
				fmt.Sprintf("user:%d:action", u.ID),
			)
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
		}

		// Добавляем кнопку отмены
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Отмена", "user:cancel"),
		})

		// Отправляем сообщение с клавиатурой
		msg := "Выберите пользователя:"
		message := tgbotapi.NewMessage(query.Message.Chat.ID, msg)
		message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}

		_, err = b.bot.Send(message)
		if err != nil {
			b.logger.Errorf("Failed to send user select message: %v", err)
		}

		b.sendCallbackResponse(query.ID, "Выберите пользователя")
		return
	}

	if action == "action" {
		// Получаем пользователя
		userID, err := strconv.ParseInt(param, 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid user ID: %v", err)
			b.sendCallbackResponse(query.ID, "Неверный ID пользователя")
			return
		}

		targetUser, err := b.repo.User().GetByID(ctx, userID)
		if err != nil {
			b.logger.Errorf("Failed to get user: %v", err)
			b.sendCallbackResponse(query.ID, "Пользователь не найден")
			return
		}

		// Формируем клавиатуру для действий с пользователем
		var keyboard [][]tgbotapi.InlineKeyboardButton

		// Кнопки для изменения роли
		if targetUser.Role != models.RoleAdmin {
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("Повысить", fmt.Sprintf("user:%d:promote", userID)),
			})
		}
		if targetUser.Role != models.RoleVassal {
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("Понизить", fmt.Sprintf("user:%d:demote", userID)),
			})
		}

		// Кнопки для отключения и удаления
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Отключить", fmt.Sprintf("user:%d:disconnect", userID)),
			tgbotapi.NewInlineKeyboardButtonData("Удалить", fmt.Sprintf("user:%d:delete", userID)),
		})

		// Кнопка отмены
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Отмена", "user:cancel"),
		})

		// Отправляем сообщение с клавиатурой
		msg := fmt.Sprintf("Действия с пользователем %s (роль: %s):", targetUser.Username, targetUser.Role)
		message := tgbotapi.NewMessage(query.Message.Chat.ID, msg)
		message.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}

		_, err = b.bot.Send(message)
		if err != nil {
			b.logger.Errorf("Failed to send user action message: %v", err)
		}

		b.sendCallbackResponse(query.ID, "Выберите действие")
		return
	}

	if action == "promote" || action == "demote" || action == "disconnect" || action == "delete" {
		// Получаем пользователя
		userID, err := strconv.ParseInt(param, 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid user ID: %v", err)
			b.sendCallbackResponse(query.ID, "Неверный ID пользователя")
			return
		}

		targetUser, err := b.repo.User().GetByID(ctx, userID)
		if err != nil {
			b.logger.Errorf("Failed to get user: %v", err)
			b.sendCallbackResponse(query.ID, "Пользователь не найден")
			return
		}

		switch action {
		case "promote":
			// Повышаем роль пользователя
			switch targetUser.Role {
			case models.RoleVassal:
				targetUser.Role = models.RoleUser
			case models.RoleUser:
				targetUser.Role = models.RoleAdmin
			default:
				b.sendCallbackResponse(query.ID, "Невозможно повысить роль пользователя")
				return
			}

			err = b.updateUserRole(ctx, targetUser)
			if err != nil {
				b.logger.Errorf("Failed to update user role: %v", err)
				b.sendCallbackResponse(query.ID, "Ошибка при обновлении роли пользователя")
				return
			}

			b.sendCallbackResponse(query.ID, "Роль пользователя повышена")
			b.sendMessage(query.Message.Chat.ID, fmt.Sprintf("Роль пользователя %s повышена до %s.", targetUser.Username, targetUser.Role))

		case "demote":
			// Понижаем роль пользователя
			switch targetUser.Role {
			case models.RoleAdmin:
				targetUser.Role = models.RoleUser
			case models.RoleUser:
				targetUser.Role = models.RoleVassal
			default:
				b.sendCallbackResponse(query.ID, "Невозможно понизить роль пользователя")
				return
			}

			err = b.updateUserRole(ctx, targetUser)
			if err != nil {
				b.logger.Errorf("Failed to update user role: %v", err)
				b.sendCallbackResponse(query.ID, "Ошибка при обновлении роли пользователя")
				return
			}

			b.sendCallbackResponse(query.ID, "Роль пользователя понижена")
			b.sendMessage(query.Message.Chat.ID, fmt.Sprintf("Роль пользователя %s понижена до %s.", targetUser.Username, targetUser.Role))

		case "disconnect":
			// Отключаем пользователя от VPN
			err = b.vpnService.DisconnectUser(ctx, userID)
			if err != nil {
				b.logger.Errorf("Failed to disconnect user: %v", err)
				b.sendCallbackResponse(query.ID, "Ошибка при отключении пользователя")
				return
			}

			b.sendCallbackResponse(query.ID, "Пользователь отключен")
			b.sendMessage(query.Message.Chat.ID, fmt.Sprintf("Пользователь %s отключен от VPN.", targetUser.Username))

		case "delete":
			// Удаляем пользователя
			err = b.repo.User().Delete(ctx, userID)
			if err != nil {
				b.logger.Errorf("Failed to delete user: %v", err)
				b.sendCallbackResponse(query.ID, "Ошибка при удалении пользователя")
				return
			}

			b.sendCallbackResponse(query.ID, "Пользователь удален")
			b.sendMessage(query.Message.Chat.ID, fmt.Sprintf("Пользователь %s удален.", targetUser.Username))
		}

		return
	}

	if action == "cancel" {
		b.sendCallbackResponse(query.ID, "Операция отменена")
		return
	}

	b.sendCallbackResponse(query.ID, "Неизвестная команда")
}

// handleTrafficCallback обрабатывает callback для действий с трафиком
func (b *TelegramBot) handleTrafficCallback(ctx context.Context, query *tgbotapi.CallbackQuery, user *models.User, param string) {
	// Получаем временной диапазон в зависимости от периода
	var from, to time.Time
	now := time.Now()

	switch param {
	case "day":
		from = now.AddDate(0, 0, -1)
		to = now
	case "week":
		from = now.AddDate(0, 0, -7)
		to = now
	case "month":
		from = now.AddDate(0, -1, 0)
		to = now
	case "year":
		from = now.AddDate(-1, 0, 0)
		to = now
	default:
		from = now.AddDate(0, 0, -30)
		to = now
	}

	// Получаем статистику трафика за указанный период
	trafficStats, err := b.vpnService.GetUserTraffic(ctx, user.ID, from.Unix(), to.Unix())
	if err != nil {
		b.logger.Errorf("Failed to get user traffic: %v", err)
		b.sendCallbackResponse(query.ID, "Ошибка при получении статистики трафика")
		return
	}

	// Формируем сообщение со статистикой
	var msg string

	switch param {
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

	b.sendCallbackResponse(query.ID, "Статистика обновлена")
}

// createInviteKeyboard создает клавиатуру для инвайт-кода
func (b *TelegramBot) createInviteKeyboard(inviteID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Удалить", fmt.Sprintf("invite:delete:%d", inviteID)),
		),
	)
}

// sendMessage отправляет текстовое сообщение
func (b *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.bot.Send(msg)
	if err != nil {
		b.logger.Errorf("Failed to send message: %v", err)
	}
}

// sendCallbackResponse отправляет ответ на callback-запрос
func (b *TelegramBot) sendCallbackResponse(callbackID string, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	b.bot.Request(callback)
}

// formatTraffic форматирует количество байт в человекочитаемый формат
func (b *TelegramBot) formatTraffic(bytes int64) string {
	return utils.FormatTraffic(bytes)
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
