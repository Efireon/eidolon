package config

// MainConfig содержит основные настройки сервиса
type MainConfig struct {
	Service ServiceConfig `yaml:"service" mapstructure:"service"`
	Logging LoggingConfig `yaml:"logging" mapstructure:"logging"`
	Storage StorageConfig `yaml:"storage" mapstructure:"storage"`
}

// ServiceConfig определяет основные параметры работы сервиса
type ServiceConfig struct {
	Host  string `yaml:"host" mapstructure:"host"`   // Хост, на котором работает сервис
	Ports []int  `yaml:"ports" mapstructure:"ports"` // Порты для прослушивания
	Name  string `yaml:"name" mapstructure:"name"`   // Имя сервиса
}

// LoggingConfig определяет настройки логирования
type LoggingConfig struct {
	Level      string         `yaml:"level" mapstructure:"level"`             // debug, info, warn, error
	Format     string         `yaml:"format" mapstructure:"format"`           // json или text
	Paths      LogPathsConfig `yaml:"paths" mapstructure:"paths"`             // Пути к файлам логов
	MaxSize    int            `yaml:"max_size" mapstructure:"max_size"`       // Максимальный размер файла логов в MB
	MaxBackups int            `yaml:"max_backups" mapstructure:"max_backups"` // Количество резервных копий логов
}

// LogPathsConfig определяет пути к различным типам логов
type LogPathsConfig struct {
	Access string `yaml:"access" mapstructure:"access"` // Путь к логам доступа
	Error  string `yaml:"error" mapstructure:"error"`   // Путь к логам ошибок
	Debug  string `yaml:"debug" mapstructure:"debug"`   // Путь к отладочным логам
}

// StorageConfig определяет настройки хранения данных
type StorageConfig struct {
	DatabasePath string           `yaml:"database_path" mapstructure:"database_path"` // Путь к файлу базы данных
	DataDir      string           `yaml:"data_dir" mapstructure:"data_dir"`           // Директория для данных
	TempDir      string           `yaml:"temp_dir" mapstructure:"temp_dir"`           // Директория для временных файлов
	BackupConfig BackupPathConfig `yaml:"backup" mapstructure:"backup"`               // Настройки резервного копирования
}

// BackupPathConfig определяет настройки резервного копирования
type BackupPathConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`         // Включено ли резервное копирование
	Path       string `yaml:"path" mapstructure:"path"`               // Путь для хранения бэкапов
	Frequency  string `yaml:"frequency" mapstructure:"frequency"`     // Частота (daily, weekly, monthly)
	MaxBackups int    `yaml:"max_backups" mapstructure:"max_backups"` // Максимальное количество бэкапов
}
