package whatsapp_handler

import (
	"github.com/gofiber/fiber/v2"
	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	log "github.com/sirupsen/logrus"
)

type WhatsappWebhookHandler struct {
	whatsappWebhookUsecase *whatsapp_usecase.WhatsappWebhookUsecase
	logger                 *customLog.Logger
}

func NewWhatsappWebhookHandler(whatsappWebhookUsecase *whatsapp_usecase.WhatsappWebhookUsecase, logger *customLog.Logger) *WhatsappWebhookHandler {
	return &WhatsappWebhookHandler{
		whatsappWebhookUsecase: whatsappWebhookUsecase,
		logger:                 logger,
	}
}

func (h *WhatsappWebhookHandler) GetWebhookURL(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	// Call usecase
	webhookURL, err := h.whatsappWebhookUsecase.GetWebhookURL(c.Context(), phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(200).JSON(fiber.Map{"url": webhookURL})
}

func (h *WhatsappWebhookHandler) SetWebhookURL(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	var req waDomain.Webhook
	if err := c.BodyParser(&req); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Call usecase
	err := h.whatsappWebhookUsecase.SetWebhookURL(c.Context(), phoneNumber, &req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(200).JSON(fiber.Map{"success": true})
}

func (h *WhatsappWebhookHandler) DeleteWebhookURL(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	// Call usecase
	err := h.whatsappWebhookUsecase.DeleteWebhookURL(c.Context(), phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(200).JSON(fiber.Map{"success": true})
}
