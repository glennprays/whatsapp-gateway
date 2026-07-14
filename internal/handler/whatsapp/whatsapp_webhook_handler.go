package whatsapp_handler

import (
	"strings"

	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
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

// splitEvents renders a stored comma-separated events filter as a JSON array.
// An empty filter (all events) becomes an empty array (never null).
func splitEvents(events string) []string {
	out := []string{}
	if strings.TrimSpace(events) == "" {
		return out
	}
	for _, e := range strings.Split(events, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// GetWebhookURL lists subscriptions. The top-level "url" is retained for
// back-compat (= first subscription's url, or "") alongside the additive
// "subscriptions" array. The HMAC secret is never returned, only has_hmac.
func (h *WhatsappWebhookHandler) GetWebhookURL(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	subs, err := h.whatsappWebhookUsecase.ListWebhookSubscriptions(c.Context(), traceID, phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	firstURL := ""
	subscriptions := make([]fiber.Map, 0, len(subs))
	for i, s := range subs {
		if i == 0 {
			firstURL = s.Url
		}
		subscriptions = append(subscriptions, fiber.Map{
			"url":      s.Url,
			"events":   splitEvents(s.Events),
			"has_hmac": s.HmacSecret != "",
		})
	}

	return c.Status(200).JSON(fiber.Map{"url": firstURL, "subscriptions": subscriptions})
}

// SetWebhookURL registers/upserts one subscription. Omitted/empty events means
// all events; an unknown event is rejected 400 in the usecase/manager.
func (h *WhatsappWebhookHandler) SetWebhookURL(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.Webhook
	if err := c.BodyParser(&req); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	err := h.whatsappWebhookUsecase.SetWebhookSubscription(c.Context(), traceID, phoneNumber, &req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(200).JSON(fiber.Map{"success": true})
}

// DeleteWebhookURL unregisters subscriptions. With no body it deletes ALL
// subscriptions (legacy semantics); with body {"url":"..."} it deletes only
// that one.
func (h *WhatsappWebhookHandler) DeleteWebhookURL(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var err error
	if body := c.Body(); len(body) > 0 {
		var req waDomain.Webhook
		if perr := c.BodyParser(&req); perr != nil {
			httpErr := httperror.FromError(perr)
			return c.Status(httpErr.Status).JSON(httpErr)
		}
		if strings.TrimSpace(req.Url) != "" {
			err = h.whatsappWebhookUsecase.DeleteWebhookSubscription(c.Context(), traceID, phoneNumber, req.Url)
		} else {
			err = h.whatsappWebhookUsecase.DeleteAllWebhookSubscriptions(c.Context(), traceID, phoneNumber)
		}
	} else {
		err = h.whatsappWebhookUsecase.DeleteAllWebhookSubscriptions(c.Context(), traceID, phoneNumber)
	}
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(200).JSON(fiber.Map{"success": true})
}
