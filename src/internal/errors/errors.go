package errors

import (
	"fmt"
)

// ModuleError представляет ошибку из конкретного модуля приложения
type ModuleError struct {
	Module  string // Название модуля, из которого пришла ошибка
	Message string // Сообщение об ошибке
	Err     error  // Исходная ошибка (опционально)
}

// Error реализует интерфейс error
func (e ModuleError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Module, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Module, e.Message)
}

// Unwrap позволяет использовать errors.Is и errors.As с вложенными ошибками
func (e ModuleError) Unwrap() error {
	return e.Err
}

// CallError создает новую ошибку модуля
func CallError(module, message string, err error) ModuleError {
	return ModuleError{
		Module:  module,
		Message: message,
		Err:     err,
	}
}

// // Обработка ошибок из разных модулей
// Обработка ошибок конфига
func CallConfigError(msg string, err error) error {
	return CallError("config", msg, err)
}

// Обработка ошибок main
func CallMainError(msg string, err error) error {
	return CallError("main", msg, err)
}

// Обработка ошибок OpenConnect
func CallOpenConnectError(msg string, err error) error {
	return CallError("openconnect", msg, err)
}

func CallUtilsError(msg string, err error) error {
	return CallError("utils", msg, err)
}
