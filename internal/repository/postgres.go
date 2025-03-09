package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"eidolon/internal/models"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Драйвер PostgreSQL
)

// PostgresRepository реализует интерфейс Repository для PostgreSQL
type PostgresRepository struct {
	db          *sqlx.DB
	userRepo    *PostgresUserRepository
	inviteRepo  *PostgresInviteRepository
	routeRepo   *PostgresRouteRepository
	trafficRepo *PostgresTrafficRepository
}

// NewPostgresRepository создает новый экземпляр PostgresRepository
func NewPostgresRepository(connectionString string) (*PostgresRepository, error) {
	db, err := sqlx.Connect("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Устанавливаем оптимальные настройки пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	repo := &PostgresRepository{
		db: db,
	}

	// Инициализируем подрепозитории
	repo.userRepo = &PostgresUserRepository{db: db}
	repo.inviteRepo = &PostgresInviteRepository{db: db}
	repo.routeRepo = &PostgresRouteRepository{db: db}
	repo.trafficRepo = &PostgresTrafficRepository{db: db}

	return repo, nil
}

// User возвращает репозиторий для работы с пользователями
func (r *PostgresRepository) User() UserRepository {
	return r.userRepo
}

// Invite возвращает репозиторий для работы с инвайт-кодами
func (r *PostgresRepository) Invite() InviteRepository {
	return r.inviteRepo
}

// Route возвращает репозиторий для работы с маршрутами
func (r *PostgresRepository) Route() RouteRepository {
	return r.routeRepo
}

// Traffic возвращает репозиторий для работы с трафиком
func (r *PostgresRepository) Traffic() TrafficRepository {
	return r.trafficRepo
}

// Close закрывает соединение с базой данных
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// PostgresUserRepository реализует UserRepository для PostgreSQL
type PostgresUserRepository struct {
	db *sqlx.DB
}

// Create создает нового пользователя
func (r *PostgresUserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (username, telegram_id, role, certificate, created_at, invited_by, traffic_limit)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	row := r.db.QueryRowContext(
		ctx, query,
		user.Username, user.TelegramID, user.Role, user.Certificate,
		user.CreatedAt, user.InvitedBy, user.TrafficLimit,
	)

	return row.Scan(&user.ID)
}

// GetByID получает пользователя по ID
func (r *PostgresUserRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT * FROM users WHERE id = $1`

	user := &models.User{}
	err := r.db.GetContext(ctx, user, query, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByTelegramID получает пользователя по Telegram ID
func (r *PostgresUserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	query := `SELECT * FROM users WHERE telegram_id = $1`

	user := &models.User{}
	err := r.db.GetContext(ctx, user, query, telegramID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByUsername получает пользователя по имени пользователя
func (r *PostgresUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT * FROM users WHERE username = $1`

	user := &models.User{}
	err := r.db.GetContext(ctx, user, query, username)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Update обновляет данные пользователя
func (r *PostgresUserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET username = $1, role = $2, certificate = $3, last_login_at = $4, traffic_limit = $5
		WHERE id = $6
	`

	result, err := r.db.ExecContext(
		ctx, query,
		user.Username, user.Role, user.Certificate, user.LastLoginAt, user.TrafficLimit, user.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

// Delete удаляет пользователя
func (r *PostgresUserRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

// List возвращает список пользователей с пагинацией
func (r *PostgresUserRepository) List(ctx context.Context, offset, limit int) ([]*models.User, error) {
	query := `SELECT * FROM users ORDER BY id LIMIT $1 OFFSET $2`

	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, limit, offset)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// CountByInviter подсчитывает количество пользователей, приглашенных указанным пользователем
func (r *PostgresUserRepository) CountByInviter(ctx context.Context, inviterID int64) (int, error) {
	query := `SELECT COUNT(*) FROM users WHERE invited_by = $1`

	var count int
	err := r.db.GetContext(ctx, &count, query, inviterID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetInvitedUsers возвращает пользователей, приглашенных указанным пользователем
func (r *PostgresUserRepository) GetInvitedUsers(ctx context.Context, inviterID int64) ([]*models.User, error) {
	query := `SELECT * FROM users WHERE invited_by = $1 ORDER BY created_at DESC`

	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, inviterID)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// PostgresInviteRepository реализует InviteRepository для PostgreSQL
type PostgresInviteRepository struct {
	db *sqlx.DB
}

// Create создает новый инвайт-код
func (r *PostgresInviteRepository) Create(ctx context.Context, invite *models.InviteCode) error {
	query := `
		INSERT INTO invite_codes (code, created_by, created_at, expires_at, expired)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	if invite.CreatedAt.IsZero() {
		invite.CreatedAt = time.Now()
	}

	if invite.ExpiresAt.IsZero() {
		// По умолчанию инвайт действителен 7 дней
		invite.ExpiresAt = time.Now().AddDate(0, 0, 7)
	}

	row := r.db.QueryRowContext(
		ctx, query,
		invite.Code, invite.CreatedBy, invite.CreatedAt, invite.ExpiresAt, invite.Expired,
	)

	return row.Scan(&invite.ID)
}

// GetByCode получает инвайт-код по коду
func (r *PostgresInviteRepository) GetByCode(ctx context.Context, code string) (*models.InviteCode, error) {
	query := `SELECT * FROM invite_codes WHERE code = $1`

	invite := &models.InviteCode{}
	err := r.db.GetContext(ctx, invite, query, code)
	if err != nil {
		return nil, err
	}

	return invite, nil
}

// GetByID получает инвайт-код по ID
func (r *PostgresInviteRepository) GetByID(ctx context.Context, id int64) (*models.InviteCode, error) {
	query := `SELECT * FROM invite_codes WHERE id = $1`

	invite := &models.InviteCode{}
	err := r.db.GetContext(ctx, invite, query, id)
	if err != nil {
		return nil, err
	}

	return invite, nil
}

// Update обновляет данные инвайт-кода
func (r *PostgresInviteRepository) Update(ctx context.Context, invite *models.InviteCode) error {
	query := `
		UPDATE invite_codes
		SET used_by = $1, used_at = $2, expired = $3
		WHERE id = $4
	`

	result, err := r.db.ExecContext(
		ctx, query,
		invite.UsedBy, invite.UsedAt, invite.Expired, invite.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("invite code not found")
	}

	return nil
}

// Delete удаляет инвайт-код
func (r *PostgresInviteRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM invite_codes WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("invite code not found")
	}

	return nil
}

// ListByCreator возвращает список инвайт-кодов, созданных указанным пользователем
func (r *PostgresInviteRepository) ListByCreator(ctx context.Context, creatorID int64) ([]*models.InviteCode, error) {
	query := `SELECT * FROM invite_codes WHERE created_by = $1 ORDER BY created_at DESC`

	invites := []*models.InviteCode{}
	err := r.db.SelectContext(ctx, &invites, query, creatorID)
	if err != nil {
		return nil, err
	}

	return invites, nil
}

// CountActiveByCreator подсчитывает количество активных инвайт-кодов, созданных указанным пользователем
func (r *PostgresInviteRepository) CountActiveByCreator(ctx context.Context, creatorID int64) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM invite_codes 
		WHERE created_by = $1 AND expired = false AND used_by = 0 AND expires_at > NOW()
	`

	var count int
	err := r.db.GetContext(ctx, &count, query, creatorID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// PostgresRouteRepository реализует RouteRepository для PostgreSQL
type PostgresRouteRepository struct {
	db *sqlx.DB
}

// Create создает новый маршрут
func (r *PostgresRouteRepository) Create(ctx context.Context, route *models.Route) error {
	query := `
		INSERT INTO routes (network, description, type, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	if route.CreatedAt.IsZero() {
		route.CreatedAt = time.Now()
	}

	row := r.db.QueryRowContext(
		ctx, query,
		route.Network, route.Description, route.Type, route.CreatedBy, route.CreatedAt,
	)

	return row.Scan(&route.ID)
}

// GetByID получает маршрут по ID
func (r *PostgresRouteRepository) GetByID(ctx context.Context, id int64) (*models.Route, error) {
	query := `SELECT * FROM routes WHERE id = $1`

	route := &models.Route{}
	err := r.db.GetContext(ctx, route, query, id)
	if err != nil {
		return nil, err
	}

	return route, nil
}

// Update обновляет данные маршрута
func (r *PostgresRouteRepository) Update(ctx context.Context, route *models.Route) error {
	query := `
		UPDATE routes
		SET network = $1, description = $2, type = $3
		WHERE id = $4
	`

	result, err := r.db.ExecContext(
		ctx, query,
		route.Network, route.Description, route.Type, route.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("route not found")
	}

	return nil
}

// Delete удаляет маршрут
func (r *PostgresRouteRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM routes WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("route not found")
	}

	return nil
}

// List возвращает список маршрутов по типу
func (r *PostgresRouteRepository) List(ctx context.Context, routeType models.RouteType) ([]*models.Route, error) {
	var query string
	var args []interface{}

	if routeType == "" {
		query = `SELECT * FROM routes ORDER BY id`
	} else {
		query = `SELECT * FROM routes WHERE type = $1 ORDER BY id`
		args = append(args, routeType)
	}

	routes := []*models.Route{}
	err := r.db.SelectContext(ctx, &routes, query, args...)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// CreateASN создает новый ASN маршрут
func (r *PostgresRouteRepository) CreateASN(ctx context.Context, route *models.ASNRoute) error {
	query := `
		INSERT INTO asn_routes (asn, description, created_by, created_at, type)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	if route.CreatedAt.IsZero() {
		route.CreatedAt = time.Now()
	}

	row := r.db.QueryRowContext(
		ctx, query,
		route.ASN, route.Description, route.CreatedBy, route.CreatedAt, route.Type,
	)

	return row.Scan(&route.ID)
}

// GetASNByID получает ASN маршрут по ID
func (r *PostgresRouteRepository) GetASNByID(ctx context.Context, id int64) (*models.ASNRoute, error) {
	query := `SELECT * FROM asn_routes WHERE id = $1`

	route := &models.ASNRoute{}
	err := r.db.GetContext(ctx, route, query, id)
	if err != nil {
		return nil, err
	}

	return route, nil
}

// ListASN возвращает список ASN маршрутов по типу
func (r *PostgresRouteRepository) ListASN(ctx context.Context, routeType models.RouteType) ([]*models.ASNRoute, error) {
	var query string
	var args []interface{}

	if routeType == "" {
		query = `SELECT * FROM asn_routes ORDER BY id`
	} else {
		query = `SELECT * FROM asn_routes WHERE type = $1 ORDER BY id`
		args = append(args, routeType)
	}

	routes := []*models.ASNRoute{}
	err := r.db.SelectContext(ctx, &routes, query, args...)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// CreateGroup создает новую группу маршрутов
func (r *PostgresRouteRepository) CreateGroup(ctx context.Context, group *models.RouteGroup) error {
	query := `
		INSERT INTO route_groups (name, description, created_by, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now()
	}

	row := r.db.QueryRowContext(
		ctx, query,
		group.Name, group.Description, group.CreatedBy, group.CreatedAt,
	)

	return row.Scan(&group.ID)
}

// GetGroupByID получает группу маршрутов по ID
func (r *PostgresRouteRepository) GetGroupByID(ctx context.Context, id int64) (*models.RouteGroup, error) {
	query := `SELECT * FROM route_groups WHERE id = $1`

	group := &models.RouteGroup{}
	err := r.db.GetContext(ctx, group, query, id)
	if err != nil {
		return nil, err
	}

	return group, nil
}

// AddRouteToGroup добавляет маршрут в группу
func (r *PostgresRouteRepository) AddRouteToGroup(ctx context.Context, groupID, routeID int64) error {
	query := `
		INSERT INTO route_group_items (group_id, route_id)
		VALUES ($1, $2)
		ON CONFLICT (group_id, route_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, groupID, routeID)
	return err
}

// RemoveRouteFromGroup удаляет маршрут из группы
func (r *PostgresRouteRepository) RemoveRouteFromGroup(ctx context.Context, groupID, routeID int64) error {
	query := `DELETE FROM route_group_items WHERE group_id = $1 AND route_id = $2`

	_, err := r.db.ExecContext(ctx, query, groupID, routeID)
	return err
}

// GetRoutesInGroup возвращает список маршрутов в группе
func (r *PostgresRouteRepository) GetRoutesInGroup(ctx context.Context, groupID int64) ([]*models.Route, error) {
	query := `
		SELECT r.* 
		FROM routes r
		JOIN route_group_items gi ON r.id = gi.route_id
		WHERE gi.group_id = $1
		ORDER BY r.id
	`

	routes := []*models.Route{}
	err := r.db.SelectContext(ctx, &routes, query, groupID)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// AssignRouteToUser связывает маршрут с пользователем
func (r *PostgresRouteRepository) AssignRouteToUser(ctx context.Context, userRoute *models.UserRoute) error {
	query := `
		INSERT INTO user_routes (user_id, route_id, enabled, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, route_id) 
		DO UPDATE SET enabled = $3
	`

	if userRoute.CreatedAt.IsZero() {
		userRoute.CreatedAt = time.Now()
	}

	_, err := r.db.ExecContext(
		ctx, query,
		userRoute.UserID, userRoute.RouteID, userRoute.Enabled, userRoute.CreatedAt,
	)

	return err
}

// GetUserRoutes возвращает список маршрутов пользователя
func (r *PostgresRouteRepository) GetUserRoutes(ctx context.Context, userID int64) ([]*models.Route, error) {
	query := `
		SELECT r.*, ur.enabled 
		FROM routes r
		JOIN user_routes ur ON r.id = ur.route_id
		WHERE ur.user_id = $1
		ORDER BY r.id
	`

	routes := []*models.Route{}
	err := r.db.SelectContext(ctx, &routes, query, userID)
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// AssignGroupToUser связывает группу маршрутов с пользователем
func (r *PostgresRouteRepository) AssignGroupToUser(ctx context.Context, userGroup *models.UserRouteGroup) error {
	query := `
		INSERT INTO user_route_groups (user_id, group_id, enabled, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, group_id) 
		DO UPDATE SET enabled = $3
	`

	if userGroup.CreatedAt.IsZero() {
		userGroup.CreatedAt = time.Now()
	}

	_, err := r.db.ExecContext(
		ctx, query,
		userGroup.UserID, userGroup.GroupID, userGroup.Enabled, userGroup.CreatedAt,
	)

	return err
}

// GetUserGroups возвращает список групп маршрутов пользователя
func (r *PostgresRouteRepository) GetUserGroups(ctx context.Context, userID int64) ([]*models.RouteGroup, error) {
	query := `
		SELECT g.*, ug.enabled 
		FROM route_groups g
		JOIN user_route_groups ug ON g.id = ug.group_id
		WHERE ug.user_id = $1
		ORDER BY g.id
	`

	groups := []*models.RouteGroup{}
	err := r.db.SelectContext(ctx, &groups, query, userID)
	if err != nil {
		return nil, err
	}

	return groups, nil
}

// UnassignRouteFromUser удаляет связь маршрута с пользователем
func (r *PostgresRouteRepository) UnassignRouteFromUser(ctx context.Context, userID, routeID int64) error {
	query := `DELETE FROM user_routes WHERE user_id = $1 AND route_id = $2`

	_, err := r.db.ExecContext(ctx, query, userID, routeID)
	return err
}

// UnassignGroupFromUser удаляет связь группы маршрутов с пользователем
func (r *PostgresRouteRepository) UnassignGroupFromUser(ctx context.Context, userID, groupID int64) error {
	query := `DELETE FROM user_route_groups WHERE user_id = $1 AND group_id = $2`

	_, err := r.db.ExecContext(ctx, query, userID, groupID)
	return err
}

// PostgresTrafficRepository реализует TrafficRepository для PostgreSQL
type PostgresTrafficRepository struct {
	db *sqlx.DB
}

// LogTraffic записывает данные о трафике пользователя
func (r *PostgresTrafficRepository) LogTraffic(ctx context.Context, traffic *models.UserTraffic) error {
	query := `
		INSERT INTO user_traffic (user_id, bytes, timestamp)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	if traffic.Timestamp.IsZero() {
		traffic.Timestamp = time.Now()
	}

	row := r.db.QueryRowContext(
		ctx, query,
		traffic.UserID, traffic.Bytes, traffic.Timestamp,
	)

	return row.Scan(&traffic.ID)
}

// GetUserTraffic возвращает данные о трафике пользователя за период
func (r *PostgresTrafficRepository) GetUserTraffic(ctx context.Context, userID int64, from, to int64) ([]*models.UserTraffic, error) {
	query := `
		SELECT * FROM user_traffic 
		WHERE user_id = $1 AND 
			timestamp >= to_timestamp($2) AND 
			timestamp <= to_timestamp($3)
		ORDER BY timestamp DESC
	`

	traffic := []*models.UserTraffic{}
	err := r.db.SelectContext(ctx, &traffic, query, userID, from, to)
	if err != nil {
		return nil, err
	}

	return traffic, nil
}

// GetTotalUserTraffic возвращает общий объем трафика пользователя
func (r *PostgresTrafficRepository) GetTotalUserTraffic(ctx context.Context, userID int64) (int64, error) {
	query := `SELECT COALESCE(SUM(bytes), 0) FROM user_traffic WHERE user_id = $1`

	var total int64
	err := r.db.GetContext(ctx, &total, query, userID)
	if err != nil {
		return 0, err
	}

	return total, nil
}
