package structures

// База OpenConnect конфига
type OpenConnectConfig struct {
	Server    string         `yaml:"server" mapstructure:"server"`
	Port      int            `yaml:"port" mapstructure:"port"`
	Protocol  string         `yaml:"protocol" mapstructure:"protocol"`   // udp/tcp
	Interface string         `yaml:"interface" mapstructure:"interface"` // tun интерфейс
	Security  SecurityConfig `yaml:"security" mapstructure:"security"`
	Network   NetworkConfig  `yaml:"network" mapstructure:"network"`
	Debug     DebugConfig    `yaml:"debug" mapstructure:"debug"`
}

// Настройки безопасности
type SecurityConfig struct {
	CertPath       string   `yaml:"cert_path" mapstructure:"cert_path"`
	KeyPath        string   `yaml:"key_path" mapstructure:"key_path"`
	CAPath         string   `yaml:"ca_path" mapstructure:"ca_path"`
	NoCertCheck    bool     `yaml:"no_cert_check" mapstructure:"no_cert_check"`
	AllowedCiphers []string `yaml:"allowed_ciphers" mapstructure:"allowed_ciphers"`
	DisableIPv6    bool     `yaml:"disable_ipv6" mapstructure:"disable_ipv6"`
}

// Настройки сети
type NetworkConfig struct {
	MTU           int      `yaml:"mtu" mapstructure:"mtu"`
	DNSServers    []string `yaml:"dns_servers" mapstructure:"dns_servers"`
	SearchDomains []string `yaml:"search_domains" mapstructure:"search_domains"`
	Routes        []string `yaml:"routes" mapstructure:"routes"`
	ExcludeRoutes []string `yaml:"exclude_routes" mapstructure:"exclude_routes"`
	DefaultRoute  bool     `yaml:"default_route" mapstructure:"default_route"`
}

// Настройки отладки
type DebugConfig struct {
	Verbose   int    `yaml:"verbose" mapstructure:"verbose"` // 0-3
	LogFile   string `yaml:"log_file" mapstructure:"log_file"`
	Timestamp bool   `yaml:"timestamp" mapstructure:"timestamp"`
	NoHTTPS   bool   `yaml:"no_https" mapstructure:"no_https"` // Для тестирования
}

// Пользовательская аутентификация
type UserAuth struct {
	Username    string
	Password    string
	Certificate *UserCertificate
}

// Пользовательские сертификаты
type UserCertificate struct {
	CertFile    string
	KeyFile     string
	KeyPassword string
}
