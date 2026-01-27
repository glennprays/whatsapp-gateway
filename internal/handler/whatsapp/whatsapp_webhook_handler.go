package whatsapp_handler

import (
	"github.com/gofiber/fiber/v2"
	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	log "github.com/sirupsen/logrus"
)

type WhatsappWebhookHandler struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
}

func NewWhatsappWebhookHandler(manager whatsapp.Manager, logger *customLog.Logger) *WhatsappWebhookHandler {
	return &WhatsappWebhookHandler{
		whatsappManager: manager,
		logger:          logger,
	}
}

func (h *WhatsappWebhookHandler) GetWebhookURL(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	webhookURL, err := h.whatsappManager.GetWebhookURL(c.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to get webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if webhookURL == nil {
		webhookURL = new(string)
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
		log.Errorf("Failed to bind JSON for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	err := h.whatsappManager.SetWebhookURL(c.Context(), phoneNumber, &req)
	if err != nil {
		log.Errorf("Failed to set webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
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

	err := h.whatsappManager.DeleteWebhookURL(c.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to delete webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(200).JSON(fiber.Map{"success": true})
}
