package config

import (
	"github.com/spf13/viper"

	// Внутренние библиотеки
	"eidolonVPN/internal/errors"
)

// Создаю переменнцю с путями конфигов TODO: Сделать более корректное сохранение путей
var paths = []string{
	"/eidolon/service/config",
}

// Объявялю струтуру ServiceConfig
type ServiceConfig struct {
	Host  string
	Ports []int
}

// Универсальная функция загрузки конфигов
func LoadConfig(configName string, paths []string, cfg interface{}) error {

	// Открываю viper
	v := viper.New()

	v.SetConfigName(configName)

	if len(paths) == 0 {
		return errors.CallConfigError("No config paths provided", nil)
	}
	for _, path := range paths {
		v.AddConfigPath(path)
	}
	return nil
}
