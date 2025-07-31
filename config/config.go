package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Port            string         `config:"PORT"`
	BasePath        string         `config:"BASE_PATH"`
	HttpOrigin      string         `config:"HTTP_ORIGIN"`
	EnableSwagger   bool           `config:"ENABLE_SWAGGER"`
	SwaggerUser     string         `config:"SWAGGER_USER"`
	SwaggerPassword string         `config:"SWAGGER_PASSWORD"`
	JwtSecret       string         `config:"JWT_SECRET"`
	JwtDuration     *time.Duration `config:"JWT_DURATION_MINUTES"`
	JwtIssuer       string         `config:"JWT_ISSUER"`
	SecretKey       string         `config:"SECRET_KEY"`
}

func LoadConfig() *Config {
	if os.Getenv("ENV") != "production" {
		err := godotenv.Load()
		if err != nil {
			log.Println("No .env file found (using system envs)")
		}
	}

	jwtTokenDurationStr := GetEnv("JWT_TOKEN_DURATION_MINUTES", "1440")
	var jwtTokenDurationTime *time.Duration
	if jwtTokenDurationStr != "0" {
		jwtTokenDuration, err := strconv.Atoi(jwtTokenDurationStr)
		if err != nil {
			log.Fatal("invalid JWT_ACCESS_TOKEN_DURATION_MINUTES:", err)
		}
		duration := time.Duration(jwtTokenDuration) * time.Minute
		jwtTokenDurationTime = &duration
	}

	return &Config{
		Port:            GetEnv("PORT", "3000"),
		BasePath:        GetEnv("BASE_PATH", "/"),
		HttpOrigin:      GetEnv("HTTP_ORIGIN", "*"),
		EnableSwagger:   GetEnv("ENABLE_SWAGGER", "true") == "true",
		SwaggerUser:     GetEnv("SWAGGER_USER", "user"),
		SwaggerPassword: GetEnv("SWAGGER_PASSWORD", "password"),
		JwtSecret:       GetEnv("JWT_SECRET", "secret"),
		JwtDuration:     jwtTokenDurationTime,
		JwtIssuer:       GetEnv("JWT_ISSUER", "whatsapp-gateway"),
		SecretKey:       GetEnv("SECRET_KEY", "secret"),
	}
}

func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
