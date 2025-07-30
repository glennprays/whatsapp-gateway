package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string `config:"PORT"`
	BasePath      string `config:"BASE_PATH"`
	HttpOrigin    string `config:"HTTP_ORIGIN"`
	EnableSwagger bool   `config:"ENABLE_SWAGGER"`
}

func LoadConfig() *Config {
	if os.Getenv("ENV") != "production" {
		err := godotenv.Load()
		if err != nil {
			log.Println("No .env file found (using system envs)")
		}
	}

	return &Config{
		Port:          GetEnv("PORT", "3000"),
		BasePath:      GetEnv("BASE_PATH", "/"),
		HttpOrigin:    GetEnv("HTTP_ORIGIN", "*"),
		EnableSwagger: GetEnv("ENABLE_SWAGGER", "true") == "true",
	}
}

func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
