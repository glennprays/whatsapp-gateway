package config

import (
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Env                                    Environment `mapstructure:"ENV" default:"development"`
	Port                                   string      `mapstructure:"PORT" default:"3000"`
	BasePath                               string      `mapstructure:"BASE_PATH" default:"/"`
	HttpOrigin                             string      `mapstructure:"HTTP_ORIGIN" default:"*"`
	EnableSwagger                          bool        `mapstructure:"ENABLE_SWAGGER" default:"true"`
	SwaggerUser                            string      `mapstructure:"SWAGGER_USER" default:"user"`
	SwaggerPassword                        string      `mapstructure:"SWAGGER_PASSWORD" default:"password"`
	SwaggerBasePath                        string      `mapstructure:"SWAGGER_BASE_PATH" default:"/docs"`
	JwtSecret                              string      `mapstructure:"JWT_SECRET" default:"secret"`
	JwtDurationMinutes                     int         `mapstructure:"JWT_TOKEN_DURATION_MINUTES" default:"1440"`
	JwtIssuer                              string      `mapstructure:"JWT_ISSUER" default:"whatsapp-gateway"`
	BasicAuthSecretKey                     string      `mapstructure:"SECRET_KEY" default:"secret"`
	WhatsappDatastoreType                  string      `mapstructure:"WHATSAPP_DATASTORE_TYPE" default:"sqlite3"`
	WhatsappDatastoreUri                   string      `mapstructure:"WHATSAPP_DATASTORE_URI" default:"file:dbs/whatsapp.db?_foreign_keys=on"`
	WhatsmeowLogLevel                      string      `mapstructure:"WHATSMEOW_LOG_LEVEL" default:"warn"`
	WhatsappDeviceLabel                    string      `mapstructure:"WHATSAPP_DEVICE_LABEL" default:"WhatsApp Gateway"`
	WhatsappWebhookHmacEncryptionMasterKey string      `mapstructure:"WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY" default:"0123456789abcdef0123456789abcdef"`
	LogLevel                               string      `mapstructure:"LOG_LEVEL" default:"debug"`
	LogOutput                              string      `mapstructure:"LOG_OUTPUT" default:"stdout"`
	LogFilePath                            string      `mapstructure:"LOG_FILE_PATH" default:"/var/log/whatsapp-gateway.log"`
	EnableCaller                           bool        `mapstructure:"LOG_ENABLE_CALLER" default:"false"`
}

type Environment string

const (
	DEV  Environment = "development"
	PROD Environment = "production"
)

func (e Environment) String() string {
	return string(e)
}

func Load() (*Config, error) {
	// Create config instance
	cfg := &Config{}

	// Apply defaults from struct tags
	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	envStr := strings.ToLower(os.Getenv("ENV"))
	env := Environment(envStr)
	if env == "" {
		env = DEV
	}

	// Load .env file
	if env == DEV {
		_ = godotenv.Load(".env")
	}

	// Configure Viper to read from environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Auto-bind each struct field by key
	t := reflect.TypeOf(cfg).Elem()
	for i := range t.NumField() {
		field := t.Field(i)
		key := field.Tag.Get("mapstructure")
		if key != "" {
			err := viper.BindEnv(key)
			if err != nil {
				return nil, err
			}
		}
	}

	// Unmarshal environment variables into config
	// This will override defaults with actual env values
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) GetJwtDuration() *time.Duration {
	d := time.Duration(c.JwtDurationMinutes) * time.Minute
	return &d
}
