FROM golang:1.24.2-alpine AS builder

WORKDIR /build

# Копируем все файлы из src в корень /build
COPY src/ ./

# Скачиваем зависимости (теперь go.mod находится в правильном месте)
RUN go mod download

# Собираем приложение из корневой директории модуля
RUN CGO_ENABLED=0 GOOS=linux go build -o eidolon ./cmd/main.go

FROM alpine:latest

COPY service/ ./eidolon/service

# Устанавливаем необходимые пакеты
RUN apk add --no-cache ca-certificates

# Устанавливаем необходимые пакеты
RUN apk add openconnect

# Создаем пользователя для безопасности
RUN adduser -D -g '' eidolon

# Даём права на работу с директорией
RUN chown -R eidolon:eidolon /eidolon/

# Копируем скомпилированный бинарник из builder stage
COPY --from=builder /build/eidolon /usr/local/bin/

# Переключаемся на созданного пользователя
USER eidolon

# Указываем команду запуска
CMD ["eidolon"]