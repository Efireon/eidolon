package handlers

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/viper"

	ers "eidolonVPN/internal/errors"
)

//// Обработка конкретных ошибок

func ConfigErrHandler(configName string, err error) error {
	switch {
	case errors.As(err, &viper.ConfigFileNotFoundError{}):
		return ers.CallConfigError(fmt.Sprintf("Config file %s not found.", configName), err)
	case errors.Is(err, fs.ErrPermission):
		return ers.CallConfigError(fmt.Sprintf("Failed to read config file %s, not enough permissions", configName), err)
	default:
		return ers.CallConfigError(fmt.Sprintf("Failed to read config file %s", configName), err)
	}
}
