package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const HMAC_SECRET = "test-secret-123"

func main() {
	http.HandleFunc("/webhook", handleWebhook)

	port := "8080"
	fmt.Printf("=================================================================\n")
	fmt.Printf("Webhook Test Server\n")
	fmt.Printf("=================================================================\n")
	fmt.Printf("Listening on: http://localhost:%s/webhook\n", port)
	fmt.Printf("HMAC Secret:  %s\n", HMAC_SECRET)
	fmt.Printf("=================================================================\n\n")
	fmt.Println("Waiting for webhooks...")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get signature header
	signature := r.Header.Get("X-Webhook-Signature")

	// Verify HMAC
	valid := verifyHMAC(body, signature, HMAC_SECRET)

	// Parse JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("[%s] ❌ Invalid JSON\n", time.Now().Format("15:04:05"))
		fmt.Printf("Body: %s\n\n", string(body))
	} else {
		// Pretty print
		fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
		fmt.Printf("[%s] 📨 Webhook Received\n", time.Now().Format("15:04:05"))
		fmt.Printf(strings.Repeat("=", 80) + "\n")

		if event, ok := payload["event"].(string); ok {
			fmt.Printf("Event: %s\n", event)
		}

		prettyJSON, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(prettyJSON))

		fmt.Printf("\nHMAC Signature: %s\n", signature)
		if valid {
			fmt.Println("✅ HMAC Valid")
		} else {
			fmt.Println("❌ HMAC Invalid")
		}
		fmt.Println(strings.Repeat("=", 80))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func verifyHMAC(body []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	providedHash := strings.TrimPrefix(signature, "sha256=")

	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedHash := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expectedHash), []byte(providedHash))
}
