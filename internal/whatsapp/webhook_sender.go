package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	log "github.com/sirupsen/logrus"
)

type WebhookSender struct {
	cipher     *cipherx.Cipher
	httpClient *http.Client
}

func NewWebhookSender(cipher *cipherx.Cipher) *WebhookSender {
	return &WebhookSender{
		cipher: cipher,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				DialContext: ssrfSafeDialContext,
			},
		},
	}
}

func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}

	for _, ip := range ips {
		if utils.IsPrivateIP(ip) {
			return nil, fmt.Errorf("blocked SSRF: resolved to private IP %s", ip)
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

func (ws *WebhookSender) Send(ctx context.Context, url string, encryptedHmacSecret string, payload interface{}) error {
	// Marshal payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Decrypt HMAC secret
	hmacSecret, err := ws.cipher.Decrypt(encryptedHmacSecret)
	if err != nil {
		log.Errorf("Failed to decrypt HMAC secret: %v", err)
		return fmt.Errorf("failed to decrypt HMAC secret: %w", err)
	}

	// Generate HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(hmacSecret))
	h.Write(jsonPayload)
	signature := hex.EncodeToString(h.Sum(nil))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", fmt.Sprintf("sha256=%s", signature))
	req.Header.Set("User-Agent", "WhatsApp-Gateway-Webhook/1.0")

	// Send request
	resp, err := ws.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}
