FROM golang:1.18-alpine AS builder

# Устанавливаем необходимые зависимости для сборки
RUN apk add --no-cache git gcc musl-dev

# Устанавливаем рабочую директорию
WORKDIR /build

# Копируем go.mod и go.sum и скачиваем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o eidolon ./cmd/server

# Финальный образ
FROM alpine:3.15

# Устанавливаем OpenConnect Server и другие необходимые пакеты
RUN apk add --no-cache ocserv openssl ca-certificates tzdata

# Создаем директории
RUN mkdir -p /app/configs /app/certs /app/logs

# Копируем бинарный файл из builder
COPY --from=builder /build/eidolon /app/eidolon

# Копируем миграции
COPY migrations /app/migrations

# Рабочая директория
WORKDIR /app

# Экспонируем порт
EXPOSE 443

# Команда запуска
CMD ["/app/eidolon", "-config", "/app/configs/config.yaml"]