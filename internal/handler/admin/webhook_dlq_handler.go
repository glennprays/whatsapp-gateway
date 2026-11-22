package admin

import (
	"errors"
	"net/http"
	"strconv"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/gofiber/fiber/v2"
)

// dlqListDefaultLimit / bounds for GET /admin/webhook-dlq pagination.
const (
	dlqDefaultLimit = 50
	dlqMinLimit     = 1
	dlqMaxLimit     = 100
)

// WebhookDLQHandler serves the operator-only webhook dead-letter plane. It is
// mounted with the admin bearer group; responses never include the encrypted
// envelope, URL, secret, raw payload, or underlying error text.
type WebhookDLQHandler struct {
	sender *whatsapp.WebhookSender
	logger *customLog.Logger
}

func NewWebhookDLQHandler(sender *whatsapp.WebhookSender, logger *customLog.Logger) *WebhookDLQHandler {
	return &WebhookDLQHandler{sender: sender, logger: logger}
}

// List returns non-sensitive dead-letter metadata, newest first.
// Defaults limit=50, accepts 1..100 and nonnegative offset; malformed -> 400.
func (h *WebhookDLQHandler) List(c *fiber.Ctx) error {
	limit := dlqDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < dlqMinLimit || n > dlqMaxLimit {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "limit must be an integer between 1 and 100"})
		}
		limit = n
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "offset must be a nonnegative integer"})
		}
		offset = n
	}

	items, err := h.sender.ListDeadLetters(c.Context(), limit, offset)
	if err != nil {
		h.logger.Error(middleware.GetTraceID(c), "Failed to list webhook dead letters", nil)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list dead letters"})
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"items":  items,
		"limit":  limit,
		"offset": offset,
	})
}

// Replay claims, decrypts and redelivers one dead letter. 404 missing/expired,
// 409 concurrently leased, 502 delivery failed, 200 delivered.
func (h *WebhookDLQHandler) Replay(c *fiber.Ctx) error {
	id := c.Params("id")
	traceID := middleware.GetTraceID(c)

	err := h.sender.ReplayDeadLetter(c.Context(), id)
	switch {
	case err == nil:
		return c.Status(http.StatusOK).JSON(fiber.Map{"id": id, "status": "delivered"})
	case errors.Is(err, whatsapp.ErrDeadLetterNotFound):
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	case errors.Is(err, whatsapp.ErrDeadLetterLeaseHeld):
		return c.Status(http.StatusConflict).JSON(fiber.Map{"error": "replay already in progress"})
	case errors.Is(err, whatsapp.ErrDeadLetterStoreDisabled):
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "dead letter store unavailable"})
	default:
		// Decrypt, transport or non-2xx failure: log classification only,
		// never the envelope or transport error text.
		h.logger.Error(traceID, "Webhook dead letter replay failed", nil, customLog.String("classification", "delivery_failed"))
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"id": id, "status": "failed"})
	}
}
