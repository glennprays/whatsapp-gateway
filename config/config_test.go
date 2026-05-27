package config

import (
	"strings"
	"testing"
)

func TestValidateProductionSecrets_RejectsDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		reject string // substring expected in error
	}{
		{
			name: "default JWT_SECRET",
			config: Config{
				JwtSecret:                              "secret",
				BasicAuthSecretKey:                     "good-key-123",
				WhatsappWebhookHmacEncryptionMasterKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1",
			},
			reject: "JWT_SECRET",
		},
		{
			name: "default SECRET_KEY",
			config: Config{
				JwtSecret:                              "good-jwt-secret",
				BasicAuthSecretKey:                     "secret",
				WhatsappWebhookHmacEncryptionMasterKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1",
			},
			reject: "SECRET_KEY",
		},
		{
			name: "default HMAC key",
			config: Config{
				JwtSecret:                              "good-jwt-secret",
				BasicAuthSecretKey:                     "good-key-123",
				WhatsappWebhookHmacEncryptionMasterKey: "0123456789abcdef0123456789abcdef",
			},
			reject: "WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validateProductionSecrets()
			if err == nil {
				t.Fatal("expected error for default secret, got nil")
			}
			if !strings.Contains(err.Error(), tt.reject) {
				t.Errorf("expected error to mention %q, got: %v", tt.reject, err)
			}
		})
	}
}

func TestValidateProductionSecrets_AcceptsNonDefaults(t *testing.T) {
	cfg := Config{
		JwtSecret:                              "my-very-secure-jwt-secret",
		BasicAuthSecretKey:                     "my-very-secure-basic-auth-key",
		WhatsappWebhookHmacEncryptionMasterKey: "abcdef1234567890abcdef1234567890",
	}

	if err := cfg.validateProductionSecrets(); err != nil {
		t.Errorf("expected no error for non-default secrets, got: %v", err)
	}
}
