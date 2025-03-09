-- Миграция для создания таблиц проекта Eidolon

-- Пользователи
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    telegram_id BIGINT NOT NULL UNIQUE,
    role VARCHAR(50) NOT NULL,
    certificate TEXT,
    created_at TIMESTAMP NOT NULL,
    last_login_at TIMESTAMP,
    invited_by BIGINT,
    traffic_limit BIGINT DEFAULT 0
);

-- Инвайт-коды
CREATE TABLE IF NOT EXISTS invite_codes (
    id SERIAL PRIMARY KEY,
    code VARCHAR(255) NOT NULL UNIQUE,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    used_by BIGINT DEFAULT 0,
    used_at TIMESTAMP,
    expired BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMP NOT NULL
);

-- Маршруты
CREATE TABLE IF NOT EXISTS routes (
    id SERIAL PRIMARY KEY,
    network VARCHAR(255) NOT NULL, -- CIDR нотация
    description VARCHAR(255),
    type VARCHAR(50) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- ASN маршруты
CREATE TABLE IF NOT EXISTS asn_routes (
    id SERIAL PRIMARY KEY,
    asn INTEGER NOT NULL,
    description VARCHAR(255),
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    type VARCHAR(50) NOT NULL
);

-- Группы маршрутов
CREATE TABLE IF NOT EXISTS route_groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(255),
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- Элементы групп маршрутов
CREATE TABLE IF NOT EXISTS route_group_items (
    group_id BIGINT NOT NULL,
    route_id BIGINT NOT NULL,
    PRIMARY KEY (group_id, route_id)
);

-- Маршруты пользователей
CREATE TABLE IF NOT EXISTS user_routes (
    user_id BIGINT NOT NULL,
    route_id BIGINT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, route_id)
);

-- Группы маршрутов пользователей
CREATE TABLE IF NOT EXISTS user_route_groups (
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, group_id)
);

-- Статистика трафика пользователей
CREATE TABLE IF NOT EXISTS user_traffic (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    bytes BIGINT NOT NULL,
    timestamp TIMESTAMP NOT NULL
);

-- Индексы
CREATE INDEX IF NOT EXISTS user_routes_user_id_idx ON user_routes (user_id);
CREATE INDEX IF NOT EXISTS user_route_groups_user_id_idx ON user_route_groups (user_id);
CREATE INDEX IF NOT EXISTS route_group_items_group_id_idx ON route_group_items (group_id);
CREATE INDEX IF NOT EXISTS user_traffic_user_id_idx ON user_traffic (user_id);
CREATE INDEX IF NOT EXISTS user_traffic_timestamp_idx ON user_traffic (timestamp);

-- Внешние ключи
ALTER TABLE users
    ADD CONSTRAINT users_invited_by_fk FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE invite_codes
    ADD CONSTRAINT invite_codes_created_by_fk FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE routes
    ADD CONSTRAINT routes_created_by_fk FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE asn_routes
    ADD CONSTRAINT asn_routes_created_by_fk FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE route_groups
    ADD CONSTRAINT route_groups_created_by_fk FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE route_group_items
    ADD CONSTRAINT route_group_items_group_id_fk FOREIGN KEY (group_id) REFERENCES route_groups(id) ON DELETE CASCADE,
    ADD CONSTRAINT route_group_items_route_id_fk FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE;

ALTER TABLE user_routes
    ADD CONSTRAINT user_routes_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT user_routes_route_id_fk FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE;

ALTER TABLE user_route_groups
    ADD CONSTRAINT user_route_groups_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT user_route_groups_group_id_fk FOREIGN KEY (group_id) REFERENCES route_groups(id) ON DELETE CASCADE;

ALTER TABLE user_traffic
    ADD CONSTRAINT user_traffic_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;