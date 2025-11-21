package whatsapp_handler

import (
	"github.com/gin-gonic/gin"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	log "github.com/sirupsen/logrus"
)

type WhatsappWebhookHandler struct {
	whatsappManager whatsapp.Manager
}

func NewWhatsappWebhookHandler(manager whatsapp.Manager) *WhatsappWebhookHandler {
	return &WhatsappWebhookHandler{
		whatsappManager: manager,
	}
}

func (h *WhatsappWebhookHandler) GetWebhookURL(c *gin.Context) {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		c.Abort()
		return
	}

	webhookURL, err := h.whatsappManager.GetWebhookURL(c.Request.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to get webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	if webhookURL == nil {
		webhookURL = new(string)
	}

	c.JSON(200, gin.H{"url": webhookURL})
}

func (h *WhatsappWebhookHandler) SetWebhookURL(c *gin.Context) {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		c.Abort()
		return
	}

	var req waDomain.Webhook
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf("Failed to bind JSON for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	err := h.whatsappManager.SetWebhookURL(c.Request.Context(), phoneNumber, &req)
	if err != nil {
		log.Errorf("Failed to set webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (h *WhatsappWebhookHandler) DeleteWebhookURL(c *gin.Context) {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		c.Abort()
		return
	}

	err := h.whatsappManager.DeleteWebhookURL(c.Request.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to delete webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	c.JSON(200, gin.H{"success": true})
}
