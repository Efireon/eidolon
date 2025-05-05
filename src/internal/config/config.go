package config

import (
	"github.com/spf13/viper"

	// Внутренние библиотеки
	"eidolonVPN/internal/errors"
	"eidolonVPN/internal/errors/handlers"
)

// Универсальная функция загрузки конфигов
func LoadConfig(configName string, paths []string, cfg interface{}) error {

	// Открываю viper
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName(configName)

	if len(paths) == 0 {
		return errors.CallConfigError("No config paths provided", nil)
	}
	for _, path := range paths {
		v.AddConfigPath(path)
	}

	err := v.ReadInConfig()
	if err != nil {
		return handlers.ConfigErrHandler(configName, err)
	}

	err = v.Unmarshal(cfg)
	if err != nil {
		return handlers.ConfigErrHandler(configName, err)
	}

	return nil
}
