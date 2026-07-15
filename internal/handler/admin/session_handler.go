package admin

import (
	"net/http"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/gofiber/fiber/v2"
)

// AdminHandler serves the operator-only admin plane. It is constructed inline in
// SetupRouter (not via Wire) and only mounted when ADMIN_API_SECRET is set. It is
// cross-tenant by design (operator visibility), gated behind the admin bearer.
type AdminHandler struct {
	manager whatsapp.Manager
	logger  *customLog.Logger
}

func NewAdminHandler(manager whatsapp.Manager, logger *customLog.Logger) *AdminHandler {
	return &AdminHandler{manager: manager, logger: logger}
}

// Sessions returns the per-instance session inventory. Empty node -> 200 with
// count 0 (never an error).
func (h *AdminHandler) Sessions(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	inv, err := h.manager.SessionInventory(c.Context(), traceID)
	if err != nil {
		h.logger.Error(traceID, "Failed to build session inventory", nil, customLog.Error(err))
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build session inventory"})
	}
	return c.Status(http.StatusOK).JSON(inv)
}

// Session returns one account by bare phone number (no normalization). 404 when
// the phone matches neither an in-memory client, a device row, nor a status row.
func (h *AdminHandler) Session(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phone := c.Params("phone")

	item, err := h.manager.GetOneSession(c.Context(), traceID, phone)
	if err != nil {
		h.logger.Error(traceID, "Failed to get session", nil, customLog.Error(err))
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get session"})
	}
	if item == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"instance": whatsapp.Hostname(),
		"session":  item,
	})
}
