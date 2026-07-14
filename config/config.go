package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Env                                    Environment `mapstructure:"ENV" default:"production"`
	Port                                   string      `mapstructure:"PORT" default:"3000"`
	BasePath                               string      `mapstructure:"BASE_PATH" default:"/"`
	HttpOrigin                             string      `mapstructure:"HTTP_ORIGIN" default:"*"`
	EnableDocumentation                    bool        `mapstructure:"ENABLE_DOCUMENTATION" default:"false"`
	DocumentationUser                      string      `mapstructure:"DOCUMENTATION_USER" default:"user"`
	DocumentationPassword                  string      `mapstructure:"DOCUMENTATION_PASSWORD" default:"password"`
	DocumentationBasePath                  string      `mapstructure:"DOCUMENTATION_BASE_PATH" default:"/docs"`
	JwtSecret                              string      `mapstructure:"JWT_SECRET" default:"secret"`
	JwtDurationMinutes                     int         `mapstructure:"JWT_TOKEN_DURATION_MINUTES" default:"1440"`
	JwtIssuer                              string      `mapstructure:"JWT_ISSUER" default:"whatsapp-gateway"`
	BasicAuthSecretKey                     string      `mapstructure:"SECRET_KEY" default:"secret"`
	WhatsappDatastoreType                  string      `mapstructure:"WHATSAPP_DATASTORE_TYPE" default:"sqlite"`
	WhatsappDatastoreUri                   string      `mapstructure:"WHATSAPP_DATASTORE_URI" default:"file:dbs/whatsapp.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"`
	WhatsmeowLogLevel                      string      `mapstructure:"WHATSMEOW_LOG_LEVEL" default:"warn"`
	WhatsappDeviceLabel                    string      `mapstructure:"WHATSAPP_DEVICE_LABEL" default:"WhatsApp Gateway"`
	WhatsappWebhookHmacEncryptionMasterKey string      `mapstructure:"WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY" default:"0123456789abcdef0123456789abcdef"`
	IncomingMessageBufferSize              int         `mapstructure:"INCOMING_MESSAGE_BUFFER_SIZE" default:"100"`
	LogLevel                               string      `mapstructure:"LOG_LEVEL" default:"info"`
	LogOutput                              string      `mapstructure:"LOG_OUTPUT" default:"stdout"`
	LogFilePath                            string      `mapstructure:"LOG_FILE_PATH" default:"/var/log/whatsapp-gateway.log"`
	EnableCaller                           bool        `mapstructure:"LOG_ENABLE_CALLER" default:"false"`

	// Database Connection Pool
	DBMaxOpenConns    int `mapstructure:"DB_MAX_OPEN_CONNS" default:"25"`
	DBMaxIdleConns    int `mapstructure:"DB_MAX_IDLE_CONNS" default:"5"`
	DBConnMaxLifeMins int `mapstructure:"DB_CONN_MAX_LIFE_MINS" default:"5"`

	// Rate Limiting Configuration
	MessageRateLimitProvider        string `mapstructure:"MESSAGE_RATE_LIMIT_PROVIDER" default:"memory"` // options: memory, redis, noop
	MessageRateLimitRequests        int64  `mapstructure:"MESSAGE_RATE_LIMIT_REQUESTS" default:"100"`
	MessageRateLimitDurationSeconds int64  `mapstructure:"MESSAGE_RATE_LIMIT_DURATION_SECONDS" default:"60"`

	// Upload limits
	MaxUploadBytes int64 `mapstructure:"MAX_UPLOAD_BYTES" default:"16777216"` // 16 MiB cap on outbound media

	// Graceful shutdown: overall bound for disconnecting all whatsmeow clients
	ShutdownClientDisconnectTimeoutSeconds int64 `mapstructure:"SHUTDOWN_CLIENT_DISCONNECT_TIMEOUT_SECONDS" default:"10"`

	// Read/query surface: server-hitting reads (groups, profiles, avatars) are
	// short-TTL cached and metered by a per-account budget so polling can't trip
	// WhatsApp anti-spam. A budget token is spent only on a cache miss.
	ReadQueryCacheTTLSeconds int64 `mapstructure:"READ_QUERY_CACHE_TTL_SECONDS" default:"300"`
	ReadQueryBudget          int64 `mapstructure:"READ_QUERY_BUDGET" default:"30"`
	ReadQueryWindowSeconds   int64 `mapstructure:"READ_QUERY_WINDOW_SECONDS" default:"60"`

	// Group & community management (Phase E). The master toggle stays ON, but the
	// high-ban-risk bulk/mass vectors (bulk participant add, join-via-link) default
	// OFF until outbound pacing (#2) lands. With GroupManagementEnabled=false the
	// entire mutation/invite/requests/community surface is unregistered (404);
	// reads (GET /group/, /group/info, /community/*) stay up.
	GroupManagementEnabled         bool `mapstructure:"GROUP_MANAGEMENT_ENABLED" default:"true"`
	GroupAddParticipantsEnabled    bool `mapstructure:"GROUP_ADD_PARTICIPANTS_ENABLED" default:"false"`
	GroupMaxParticipantsPerRequest int  `mapstructure:"GROUP_MAX_PARTICIPANTS_PER_REQUEST" default:"256"`
	GroupJoinViaLinkEnabled        bool `mapstructure:"GROUP_JOIN_VIA_LINK_ENABLED" default:"false"`

	// Send idempotency: an optional Idempotency-Key header dedupes sends via a
	// DB-backed (phone, key) table. TTL bounds how long a completed response is
	// replayable; PendingTimeout lets a retry take over a row left pending by a
	// crashed request.
	IdempotencyTTLSeconds            int64 `mapstructure:"IDEMPOTENCY_TTL_SECONDS" default:"86400"`
	IdempotencyPendingTimeoutSeconds int64 `mapstructure:"IDEMPOTENCY_PENDING_TIMEOUT_SECONDS" default:"30"`

	// Admin plane: operator-only, cross-tenant endpoints (/admin/*, /metrics) at
	// the ROOT path. Empty secret keeps the whole plane unregistered (404, dark
	// by default); when set, requests need Authorization: Bearer <secret>.
	AdminAPISecret string `mapstructure:"ADMIN_API_SECRET" default:""`
	// MetricsEnabled toggles /metrics independently; it still requires
	// ADMIN_API_SECRET to be set to be reachable (same bearer-gated plane).
	MetricsEnabled bool `mapstructure:"METRICS_ENABLED" default:"false"`

	// Register endpoint rate limiting (per-IP, in-process memory limiter)
	RegisterRateLimitEnabled         bool  `mapstructure:"REGISTER_RATE_LIMIT_ENABLED" default:"true"`
	RegisterRateLimitRequests        int64 `mapstructure:"REGISTER_RATE_LIMIT_REQUESTS" default:"5"`
	RegisterRateLimitDurationSeconds int64 `mapstructure:"REGISTER_RATE_LIMIT_DURATION_SECONDS" default:"60"`

	// RabbitMQ Configuration
	RabbitMQEnabled               bool   `mapstructure:"RABBITMQ_ENABLED" default:"false"`
	RabbitMQURL                   string `mapstructure:"RABBITMQ_URL" default:"amqp://user:user@localhost:5672/"`
	RabbitMQConnectionName        string `mapstructure:"RABBITMQ_CONNECTION_NAME" default:"whatsapp-gateway"`
	RabbitMQPrefetchCount         int    `mapstructure:"RABBITMQ_PREFETCH_COUNT" default:"5"`
	RabbitMQReconnectDelaySeconds int    `mapstructure:"RABBITMQ_RECONNECT_DELAY_SECONDS" default:"5"`
	RabbitMQPublishConfirm        bool   `mapstructure:"RABBITMQ_PUBLISH_CONFIRM" default:"true"`
	RabbitMQConfirmTimeoutSeconds int    `mapstructure:"RABBITMQ_CONFIRM_TIMEOUT_SECONDS" default:"5"`

	// Redis Configuration
	RedisEnabled bool   `mapstructure:"REDIS_ENABLED" default:"false"`
	RedisURI     string `mapstructure:"REDIS_URI" default:"redis://localhost:6379/0"`

	// Worker Pool Sizes
	WorkerIncomingEvents   int `mapstructure:"WORKER_INCOMING_EVENTS" default:"5"`
	WorkerWebhookDelivery  int `mapstructure:"WORKER_WEBHOOK_DELIVERY" default:"10"`
	WorkerOutgoingMessages int `mapstructure:"WORKER_OUTGOING_MESSAGES" default:"3"`

	// Queue Retry Settings
	QueueMaxRetries int `mapstructure:"QUEUE_MAX_RETRIES" default:"3"`

	// Queue Duplicate Detection (in-memory, single instance only)
	QueueDedupEnabled    bool `mapstructure:"QUEUE_DEDUP_ENABLED" default:"true"`
	QueueDedupTTLSeconds int  `mapstructure:"QUEUE_DEDUP_TTL_SECONDS" default:"600"`

	// Status Webhook Configuration. WebhookStatusEventsEnabled is the master
	// kill-switch over the message.queued/sent/failed family.
	WebhookStatusEventsEnabled bool `mapstructure:"WEBHOOK_STATUS_EVENTS_ENABLED" default:"true"`
	// Deprecated: superseded by the per-subscription events filter (POST
	// /webhook). Retained so existing .env files still parse; no longer applied
	// as a delivery filter.
	WebhookStatusEvents string `mapstructure:"WEBHOOK_STATUS_EVENTS" default:"message.sent,message.failed"`

	// Direct-mode webhook retry parity: direct-mode status webhooks are delivered
	// asynchronously with bounded exponential backoff (queue mode keeps RabbitMQ
	// retry). Backoff is the base for the exponential schedule.
	WebhookMaxRetries          int   `mapstructure:"WEBHOOK_MAX_RETRIES" default:"3"`
	WebhookRetryBackoffSeconds int64 `mapstructure:"WEBHOOK_RETRY_BACKOFF_SECONDS" default:"2"`

	// Storage Configuration
	// Both providers are production-ready - choose based on infrastructure needs
	StorageProvider             string `mapstructure:"STORAGE_PROVIDER" default:"local"` // options: s3, local
	StorageS3Endpoint           string `mapstructure:"STORAGE_S3_ENDPOINT" default:"s3.amazonaws.com"`
	StorageS3AccessKeyID        string `mapstructure:"STORAGE_S3_ACCESS_KEY_ID" default:""`
	StorageS3SecretAccessKey    string `mapstructure:"STORAGE_S3_SECRET_ACCESS_KEY" default:""`
	StorageS3Region             string `mapstructure:"STORAGE_S3_REGION" default:"us-east-1"`
	StorageS3Bucket             string `mapstructure:"STORAGE_S3_BUCKET" default:"whatsapp-gateway"`
	StorageS3UseSSL             bool   `mapstructure:"STORAGE_S3_USE_SSL" default:"true"`
	StorageS3PresignedURLExpiry int64  `mapstructure:"STORAGE_S3_PRESIGNED_URL_EXPIRY_SECONDS" default:"86400"`
	StorageLocalPath            string `mapstructure:"STORAGE_LOCAL_PATH" default:"./storage"`
	StorageBaseURL              string `mapstructure:"STORAGE_BASE_URL" default:""`
	StorageAPIPath              string `mapstructure:"STORAGE_API_PATH" default:"/storage"`

	// Webhook Media Configuration
	WebhookMediaDownloadEnabled        bool   `mapstructure:"WEBHOOK_MEDIA_DOWNLOAD_ENABLED" default:"false"`
	WebhookMediaStoragePrefix          string `mapstructure:"WEBHOOK_MEDIA_STORAGE_PREFIX" default:"webhook/media"`
	WebhookMediaDownloadTimeoutSeconds int64  `mapstructure:"WEBHOOK_MEDIA_DOWNLOAD_TIMEOUT_SECONDS" default:"30"`
	WebhookMediaFallbackToWhatsAppURL  bool   `mapstructure:"WEBHOOK_MEDIA_FALLBACK_TO_WHATSAPP_URL" default:"true"`
	WebhookMediaRetentionDays          int    `mapstructure:"WEBHOOK_MEDIA_RETENTION_DAYS" default:"30"`
	WebhookMediaMaxFileSizeBytes       int64  `mapstructure:"WEBHOOK_MEDIA_MAX_FILE_SIZE_BYTES" default:"10485760"` // 10MB default

	// Storage Lifecycle Configuration
	StorageAutoDeleteEnabled       bool `mapstructure:"STORAGE_AUTO_DELETE_ENABLED" default:"false"`
	StorageAutoDeleteIntervalHours int  `mapstructure:"STORAGE_AUTO_DELETE_INTERVAL_HOURS" default:"24"`
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

	if cfg.Env == PROD {
		if err := cfg.validateProductionSecrets(); err != nil {
			return nil, err
		}
	}

	cfg.normalize()

	return cfg, nil
}

// maxJWTDurationMinutes caps token lifetime at 1 year. JWT_TOKEN_DURATION_MINUTES
// is multiplied by time.Minute (nanoseconds) to build the token expiry; an
// out-of-range value (e.g. the 1e18 seen in a committed .env) overflows
// time.Duration and yields a garbage/near-infinite expiry. Clamp it.
const maxJWTDurationMinutes = 525600 // 1 year

// normalize clamps derived config values into safe ranges after env loading.
func (c *Config) normalize() {
	if c.JwtDurationMinutes <= 0 || c.JwtDurationMinutes > maxJWTDurationMinutes {
		c.JwtDurationMinutes = 1440 // 24h default
	}
}

func (c *Config) validateProductionSecrets() error {
	defaults := map[string]string{
		"JWT_SECRET": "secret",
		"SECRET_KEY": "secret",
		"WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY": "0123456789abcdef0123456789abcdef",
	}
	values := map[string]string{
		"JWT_SECRET": c.JwtSecret,
		"SECRET_KEY": c.BasicAuthSecretKey,
		"WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY": c.WhatsappWebhookHmacEncryptionMasterKey,
	}
	for key, val := range values {
		if val == defaults[key] {
			return fmt.Errorf("refusing to start in production: %s is set to its default value", key)
		}
	}
	return nil
}

func (c *Config) GetJwtDuration() *time.Duration {
	d := time.Duration(c.JwtDurationMinutes) * time.Minute
	return &d
}

func (c *Config) GetRateLimitDuration() time.Duration {
	d := time.Duration(c.MessageRateLimitDurationSeconds) * time.Second
	return d
}
