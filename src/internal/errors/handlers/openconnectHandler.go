package handlers

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"syscall"

	ers "eidolonVPN/internal/errors"
)

// Обработка специфичных ошибок OpenConnect
func OpenConnectErrHandler(err error) error {
	// Сначала проверяем на системные ошибки
	switch {
	case errors.Is(err, fs.ErrPermission):
		return ers.CallOpenConnectError("Permission denied: OpenConnect requires root privileges", err)

	case errors.Is(err, syscall.EADDRINUSE):
		return ers.CallOpenConnectError("Address already in use", err)

	case errors.Is(err, syscall.ECONNREFUSED):
		return ers.CallOpenConnectError("Connection refused: VPN server not available", err)

	case errors.Is(err, os.ErrNotExist):
		return ers.CallOpenConnectError("OpenConnect binary or config file not found", err)

	case errors.Is(err, syscall.ETIMEDOUT):
		return ers.CallOpenConnectError("Connection timeout: VPN server did not respond", err)

	default:
		return ers.CallOpenConnectError(fmt.Sprintf("Unexpected error: %v", err), err)
	}
}

func OpenConnectConfigErrHandler(occfg string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Fatalf("Critical: failed to load yaml")
		return nil
	case errors.Is(err, fs.ErrPermission):
		return ers.CallOpenConnectError("OpenConnect not enough permissions", err)
	default:
		return ers.CallOpenConnectError(fmt.Sprintf("Unexpected error: %v", err), err)
	}
}
