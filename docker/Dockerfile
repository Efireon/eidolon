# Первый этап - Go билдер 
FROM golang:1.24.2-alpine AS go-builder

WORKDIR /build
COPY src/ ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o eidolon ./cmd/main.go

# Второй этап - компиляция ocserv
FROM alpine:latest AS ocserv-builder
RUN apk add --no-cache build-base gnutls-dev libev-dev \
    libseccomp-dev linux-headers readline-dev libnl3-dev \
    gnutls-utils protobuf-c-dev zlib-dev lz4-dev

WORKDIR /build
RUN wget https://www.infradead.org/ocserv/download/ocserv-1.3.0.tar.xz && \
    tar -xf ocserv-*.tar.xz && \
    cd ocserv-* && \
    ./configure && \
    make && \
    make install DESTDIR=/install

# Финальный этап - собираем всё вместе
FROM alpine:latest

# Устанавливаем необходимые пакеты
RUN apk add --no-cache ca-certificates \
    gnutls \
    gnutls-utils \
    libev \
    libseccomp \
    libnl3 \
    protobuf-c \
    lz4-libs \
    readline \
    openssl \
    linux-pam

# Создаем пользователя для безопасности
RUN adduser -D -g '' eidolon

# Копируем eidolon из Go-билдера
COPY --from=go-builder /build/eidolon /usr/local/bin/

# Копируем ocserv из ocserv-билдера
COPY --from=ocserv-builder /install/usr/local/sbin/ocserv /usr/local/sbin/
COPY --from=ocserv-builder /install/usr/local/bin/ocpasswd /usr/local/bin/

# Копируем конфигурационные файлы
COPY service/ ./eidolon/service

# Даём права на работу с директорией
RUN chown -R eidolon:eidolon /eidolon/

# Переключаемся на созданного пользователя
USER eidolon

# Указываем команду запуска
CMD ["eidolon"]