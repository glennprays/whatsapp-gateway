package whatsapp_handler

import (
	"net/http"

	customLog "github.com/glennprays/log"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// badRequest logs a body-parse failure and writes the 400 response.
func (h *WhatsappMessageHandler) badRequest(c *fiber.Ctx, traceID string, err error) error {
	h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
	httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
	return c.Status(httpErr.Status).JSON(httpErr)
}

func (h *WhatsappMessageHandler) CreateGroup(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.CreateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.CreateGroup(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusCreated).JSON(resp)
}

func (h *WhatsappMessageHandler) LeaveGroup(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.GroupTargetRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	if err := h.whatsappMessageUsecase.LeaveGroup(c.Context(), traceID, phoneNumber, req); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"left": true})
}

func (h *WhatsappMessageHandler) UpdateGroupParticipants(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.GroupParticipantsRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.UpdateGroupParticipants(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) SetGroupSettings(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.GroupSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.SetGroupSettings(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) SetGroupName(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SetGroupNameRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	if err := h.whatsappMessageUsecase.SetGroupName(c.Context(), traceID, phoneNumber, req); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"updated": true})
}

func (h *WhatsappMessageHandler) SetGroupTopic(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SetGroupTopicRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	if err := h.whatsappMessageUsecase.SetGroupTopic(c.Context(), traceID, phoneNumber, req); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"updated": true})
}

func (h *WhatsappMessageHandler) SetGroupPhoto(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	fileHeader, err := c.FormFile("photo")
	if err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.SetGroupPhoto(c.Context(), traceID, phoneNumber,
		c.FormValue("chat"), c.FormValue("msisdn"), fileHeader)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) RemoveGroupPhoto(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	// Accept {chat} in the body, falling back to ?chat=/?msisdn= for callers that
	// can't send a DELETE body.
	var req waDomain.GroupTargetRequest
	_ = c.BodyParser(&req)
	if req.Chat == "" && req.Msisdn == "" {
		req.Chat = c.Query("chat")
		req.Msisdn = c.Query("msisdn")
	}

	resp, err := h.whatsappMessageUsecase.RemoveGroupPhoto(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) GetGroupInviteLink(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	resp, err := h.whatsappMessageUsecase.GetGroupInviteLink(c.Context(), traceID, phoneNumber,
		c.Query("chat"), c.Query("msisdn"))
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) ResetGroupInviteLink(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.GroupInviteResetRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.ResetGroupInviteLink(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) GetGroupInviteInfo(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	resp, err := h.whatsappMessageUsecase.GetGroupInviteInfo(c.Context(), traceID, phoneNumber, c.Query("code"))
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) JoinGroup(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.JoinGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.JoinGroup(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) ListJoinRequests(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	resp, err := h.whatsappMessageUsecase.ListJoinRequests(c.Context(), traceID, phoneNumber,
		c.Query("chat"), c.Query("msisdn"))
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) UpdateJoinRequests(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.GroupJoinRequestsActionRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	resp, err := h.whatsappMessageUsecase.UpdateJoinRequests(c.Context(), traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) LinkSubGroup(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.CommunityLinkRequest
	if err := c.BodyParser(&req); err != nil {
		return h.badRequest(c, traceID, err)
	}

	if err := h.whatsappMessageUsecase.LinkSubGroup(c.Context(), traceID, phoneNumber, req); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"linked": true})
}

func (h *WhatsappMessageHandler) UnlinkSubGroup(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	if err := h.whatsappMessageUsecase.UnlinkSubGroup(c.Context(), traceID, phoneNumber,
		c.Query("chat"), c.Query("msisdn"), c.Query("child")); err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"unlinked": true})
}
