package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"eidolon/internal/models"
	"eidolon/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// TelegramBot представляет бота для управления VPN через Telegram
type TelegramBot struct {
	bot           *tgbotapi.BotAPI
	authService   *service.AuthService
	inviteService *service.InviteService
	vpnService    *service.VPNService
	logger        *logrus.Logger
	admins        []int64 // Список Telegram ID администраторов для первоначальной настройки
}

// NewTelegramBot создает нового Telegram бота
func NewTelegramBot(
	token string,
	authService *service.AuthService,
	inviteService *service.InviteService,
	vpnService *service.VPNService,
	logger *logrus.Logger,
	admins []int64,
) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	return &TelegramBot{
		bot:           bot,
		authService:   authService,
		inviteService: inviteService,
		vpnService:    vpnService,
		logger:        logger,
		admins:        admins,
	}, nil
}

// Start запускает бота
func (b *TelegramBot) Start(ctx context.Context) error {
	b.logger.Info("Starting Telegram bot...")

	// Настраиваем обновления
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Получаем канал обновлений
	updates := b.bot.GetUpdatesChan(u)

	// Обрабатываем обновления
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Stopping Telegram bot...")
			b.bot.StopReceivingUpdates()
			return nil
		case update := <-updates:
			go b.handleUpdate(ctx, update)
		}
	}
}

// handleUpdate обрабатывает обновление от Telegram
func (b *TelegramBot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Errorf("Recovered from panic in handleUpdate: %v", r)
		}
	}()

	// Обрабатываем сообщения
	if update.Message != nil {
		b.handleMessage(ctx, update.Message)
		return
	}

	// Обрабатываем callback-запросы (нажатия на кнопки инлайн-клавиатуры)
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(ctx, update.CallbackQuery)
		return
	}
}

// handleMessage обрабатывает сообщение от пользователя
func (b *TelegramBot) handleMessage(ctx context.Context, message *tgbotapi.Message) {
	// Игнорируем сообщения от ботов
	if message.From.IsBot {
		return
	}

	// Получаем или регистрируем пользователя
	user, err := b.authService.AuthenticateWithTelegram(ctx, int64(message.From.ID))
	if err != nil {
		// Если пользователь не найден, регистрируем его
		if err == service.ErrUserNotFound {
			user, err = b.authService.RegisterUserWithTelegram(ctx, int64(message.From.ID), message.From.UserName)
			if err != nil {
				b.logger.Errorf("Failed to register user: %v", err)
				b.sendMessage(message.Chat.ID, "Ошибка при регистрации. Пожалуйста, попробуйте позже.")
				return
			}

			// Проверяем, является ли пользователь администратором (при первоначальной настройке)
			for _, adminID := range b.admins {
				if adminID == int64(message.From.ID) {
					// Устанавливаем роль админа
					user.Role = models.RoleAdmin
					err = b.updateUserRole(ctx, user)
					if err != nil {
						b.logger.Errorf("Failed to set admin role: %v", err)
					}
					break
				}
			}

			// Отправляем приветственное сообщение
			welcomeMsg := "Добро пожаловать в Eidolon VPN!\n\n"

			if user.Role == models.RoleAdmin {
				welcomeMsg += "Вы зарегистрированы как администратор.\n"
			} else {
				welcomeMsg += "Для использования VPN вам необходимо ввести инвайт-код.\n"
				welcomeMsg += "Используйте команду /invite [код] для активации.\n"
			}

			welcomeMsg += "\nДля получения списка команд, отправьте /help."

			b.sendMessage(message.Chat.ID, welcomeMsg)
			return
		}

		b.logger.Errorf("Authentication error: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка аутентификации. Пожалуйста, попробуйте позже.")
		return
	}

	// Обрабатываем команды
	if message.IsCommand() {
		b.handleCommand(ctx, message, user)
		return
	}

	// Если это не команда, отправляем справку
	b.sendHelp(message.Chat.ID, user)
}

// handleCommand обрабатывает команду от пользователя
func (b *TelegramBot) handleCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) {
	command := message.Command()
	args := message.CommandArguments()

	switch command {
	case "start":
		b.sendMessage(message.Chat.ID, "Добро пожаловать в Eidolon VPN!\nДля получения списка команд, отправьте /help.")

	case "help":
		b.sendHelp(message.Chat.ID, user)

	case "status":
		b.handleStatusCommand(ctx, message.Chat.ID, user)

	case "invite":
		b.handleInviteCommand(ctx, message.Chat.ID, user, args)

	case "generate":
		b.handleGenerateCommand(ctx, message.Chat.ID, user)

	case "myinvites":
		b.handleMyInvitesCommand(ctx, message.Chat.ID, user)

	case "routes":
		b.handleRoutesCommand(ctx, message.Chat.ID, user)

	case "addroute":
		b.handleAddRouteCommand(ctx, message.Chat.ID, user, args)

	case "traffic":
		b.handleTrafficCommand(ctx, message.Chat.ID, user)

	case "disconnect":
		b.handleDisconnectCommand(ctx, message.Chat.ID, user, args)

	case "users":
		b.handleUsersCommand(ctx, message.Chat.ID, user, args)

	case "config":
		b.handleConfigCommand(ctx, message.Chat.ID, user)

	default:
		b.sendMessage(message.Chat.ID, "Неизвестная команда. Отправьте /help для получения списка команд.")
	}
}

// handleCallbackQuery обрабатывает нажатия на кнопки инлайн-клавиатуры
func (b *TelegramBot) handleCallbackQuery(ctx context.Context, query *tgbotapi.CallbackQuery) {
	// Получаем пользователя
	user, err := b.authService.AuthenticateWithTelegram(ctx, int64(query.From.ID))
	if err != nil {
		b.logger.Errorf("Authentication error in callback: %v", err)
		return
	}

	// Обрабатываем callback данные
	data := query.Data
	parts := strings.Split(data, ":")

	if len(parts) < 2 {
		return
	}

	action := parts[0]
	param := parts[1]

	switch action {
	case "route":
		b.handleRouteCallback(ctx, query, user, param)

	case "group":
		b.handleGroupCallback(ctx, query, user, param)

	case "invite":
		b.handleInviteCallback(ctx, query, user, param)

	case "user":
		if len(parts) >= 3 {
			b.handleUserCallback(ctx, query, user, param, parts[2])
		}
	}

	// Отвечаем на callback, чтобы убрать "часы" у кнопки
	callback := tgbotapi.NewCallback(query.ID, "")
	b.bot.Request(callback)
}

// sendHelp отправляет список доступных команд
func (b *TelegramBot) sendHelp(chatID int64, user *models.User) {
	helpMsg := "Доступные команды:\n\n"
	helpMsg += "/status - Показать статус VPN\n"
	helpMsg += "/invite [код] - Активировать инвайт-код\n"
	helpMsg += "/traffic - Показать статистику трафика\n"
	helpMsg += "/config - Получить конфигурацию VPN\n"

	// Команды для пользователей с ролью user и admin
	if user.Role == models.RoleUser || user.Role == models.RoleAdmin {
		helpMsg += "/generate - Сгенерировать инвайт-код\n"
		helpMsg += "/myinvites - Показать мои инвайт-коды\n"
	}

	// Команды для пользователей с возможностью добавлять маршруты
	userLimits := user.GetRoleLimits()
	if userLimits.CanAddRoutes {
		helpMsg += "/routes - Управление маршрутами\n"
		helpMsg += "/addroute [сеть CIDR] - Добавить маршрут\n"
	}

	// Команды только для администраторов
	if user.Role == models.RoleAdmin {
		helpMsg += "/users [параметры] - Управление пользователями\n"
		helpMsg += "/disconnect [имя пользователя] - Отключить пользователя\n"
	}

	b.sendMessage(chatID, helpMsg)
}

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
		traffic := formatTraffic(totalTraffic)
		statusMsg += fmt.Sprintf("Использовано трафика: %s\n", traffic)
	}

	b.sendMessage(chatID, statusMsg)
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

// handleGenerateCommand обрабатывает команду /generate
func (b *TelegramBot) handleGenerateCommand(ctx context.Context, chatID int64, user *models.User) {
	// Проверяем, что пользователь имеет право генерировать инвайт-коды
	userLimits := user.GetRoleLimits()
	if userLimits.MaxInvites == 0 {
		b.sendMessage(chatID, "У вас нет прав на генерацию инвайт-кодов.")
		return
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

	_, err = b.bot.Send(message)
	if err != nil {
		b.logger.Errorf("Failed to send message: %v", err)
		b.sendMessage(chatID, "Ошибка при отправке списка инвайт-кодов.")
	}
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

	b.sendMessage(chatID, msg)
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

	// Создаем новый маршрут
	route := &models.Route{
		Network:     args,
		Description: "Добавлен через Telegram",
		Type:        models.RouteTypeCustom,
		CreatedBy:   user.ID,
		CreatedAt:   time.Now(),
	}

	// Добавляем маршрут
	err := b.vpnService.CreateRoute(ctx, route)
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

	b.sendMessage(chatID, fmt.Sprintf("Маршрут %s успешно добавлен!", args))
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
		totalTraffic := formatTraffic(totalBytes)
		msg += fmt.Sprintf("Общий трафик за 30 дней: %s\n\n", totalTraffic)

		// Получаем суточную статистику
		dailyStats := aggregateDailyTraffic(trafficStats)

		// Выводим статистику по дням (последние 7 дней)
		days := 0
		for date, bytes := range dailyStats {
			if days >= 7 {
				break
			}
			traffic := formatTraffic(bytes)
			msg += fmt.Sprintf("%s: %s\n", date, traffic)
			days++
		}
	}

	b.sendMessage(chatID, msg)
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

	b.sendMessage(chatID, msg)
}
