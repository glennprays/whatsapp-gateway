package whatsapp_handler

import (
	"net/http"
	"strconv"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// GetIncomingMessages returns the most recent incoming messages for the
// authenticated phone. Query param `limit` defaults to 10 and is clamped
// to [1, 50] by the usecase layer. Messages are returned newest first.
func (h *WhatsappMessageHandler) GetIncomingMessages(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	limit, _ := strconv.Atoi(c.Query("limit"))

	msgs, err := h.whatsappMessageUsecase.GetIncomingMessages(c.Context(), traceID, phoneNumber, limit)
	if err != nil {
		h.logger.Error(traceID, "Failed to get incoming messages", []customLog.Field{customLog.Error(err)})
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success":   true,
		"timestamp": time.Now().UnixMilli(),
		"count":     len(msgs),
		"messages":  msgs,
	})
}
