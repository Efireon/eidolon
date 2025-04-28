# Создаём образ с пресетом Golang
FROM golang:1.24.2-alpine AS builder


WORKDIR /build

COPY src/go.mod src/go.sum ./
COPY src/ ./src/

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o eidolon ./src/cmd/main.go

FROM alpine:latest

# Устанавливаем необходимые пакеты
RUN apk add --no-cache ca-certificates

# Создаем пользователя для безопасности
RUN adduser -D -g '' eidolon

# Копируем скомпилированный бинарник из builder stage
COPY --from=builder /build/eidolon /usr/local/bin/

# Переключаемся на созданного пользователя
USER eidolon

# Указываем команду запуска
CMD ["eidolon"]