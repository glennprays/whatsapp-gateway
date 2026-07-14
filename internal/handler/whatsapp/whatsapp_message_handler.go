package whatsapp_handler

import (
	"errors"
	"net/http"
	"strconv"

	customLog "github.com/glennprays/log"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
)

type WhatsappMessageHandler struct {
	whatsappMessageUsecase *whatsapp_usecase.WhatsappMessageUsecase
	logger                 *customLog.Logger
}

func NewWhatsappMessageHandler(whatsappMessageUsecase *whatsapp_usecase.WhatsappMessageUsecase, logger *customLog.Logger) *WhatsappMessageHandler {
	return &WhatsappMessageHandler{
		whatsappMessageUsecase: whatsappMessageUsecase,
		logger:                 logger,
	}
}

func (h *WhatsappMessageHandler) SendTextMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendTextMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()

	// Call usecase
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendTextMessage(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Return queued response if available
	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}

	// Return direct response
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) SendImageMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	// Parse multipart form
	var req waDomain.SendImageMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse form data", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Get image file
	fileHeader, err := c.FormFile("image")
	if err != nil {
		h.logger.Error(traceID, "Failed to get image file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Parse is_view_once
	isViewOnceStr := c.FormValue("is_view_once")
	isViewOnce := false
	if isViewOnceStr != "" {
		isViewOnce, _ = strconv.ParseBool(isViewOnceStr)
	}

	ctx := c.Context()

	// Call usecase
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendImageMessage(ctx, traceID, phoneNumber, req, fileHeader, isViewOnce)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Return queued response if available
	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}

	// Return direct response
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) SendDocumentMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendDocumentMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse form data", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	fileHeader, err := c.FormFile("document")
	if err != nil {
		h.logger.Error(traceID, "Failed to get document file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendDocumentMessage(ctx, traceID, phoneNumber, req, fileHeader)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) SendVideoMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendVideoMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse form data", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	fileHeader, err := c.FormFile("video")
	if err != nil {
		h.logger.Error(traceID, "Failed to get video file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	req.IsGif, _ = strconv.ParseBool(c.FormValue("is_gif"))
	req.IsViewOnce, _ = strconv.ParseBool(c.FormValue("is_view_once"))

	ctx := c.Context()
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendVideoMessage(ctx, traceID, phoneNumber, req, fileHeader)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) SendAudioMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendAudioMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse form data", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	fileHeader, err := c.FormFile("audio")
	if err != nil {
		h.logger.Error(traceID, "Failed to get audio file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	isPTT, _ := strconv.ParseBool(c.FormValue("is_ptt"))
	isViewOnce, _ := strconv.ParseBool(c.FormValue("is_view_once"))
	req.IsPTT = isPTT
	req.IsViewOnce = isViewOnce

	ctx := c.Context()
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendAudioMessage(ctx, traceID, phoneNumber, req, fileHeader)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) ReactToMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.MessageReactionRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()

	// Call usecase
	err := h.whatsappMessageUsecase.ReactToMessage(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(waDomain.MessageOperationResponse{
		Success: true,
	})
}

func (h *WhatsappMessageHandler) DeleteMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.MessageDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()

	// Call usecase
	err := h.whatsappMessageUsecase.DeleteMessage(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(waDomain.MessageOperationResponse{
		Success: true,
	})
}

func (h *WhatsappMessageHandler) EditMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.MessageEditRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()

	// Call usecase
	err := h.whatsappMessageUsecase.EditMessage(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(waDomain.MessageOperationResponse{
		Success: true,
	})
}

func (h *WhatsappMessageHandler) SendLocationMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendLocationMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendLocationMessage(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) SendPollMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendPollMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse request body", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendPollMessage(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) SendStickerMessage(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.SendStickerMessageRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse form data", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	fileHeader, err := c.FormFile("sticker")
	if err != nil {
		h.logger.Error(traceID, "Failed to get sticker file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()
	directResponse, queuedResponse, err := h.whatsappMessageUsecase.SendStickerMessage(ctx, traceID, phoneNumber, req, fileHeader)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if queuedResponse != nil {
		return c.Status(http.StatusAccepted).JSON(queuedResponse)
	}
	return c.Status(http.StatusOK).JSON(directResponse)
}

func (h *WhatsappMessageHandler) CheckNumber(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	var req waDomain.ContactCheckRequest
	if err := c.QueryParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to parse query", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	if req.Msisdn == "" && req.Chat == "" {
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, errors.New("chat (or msisdn) query param is required")))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()
	resp, err := h.whatsappMessageUsecase.CheckNumber(ctx, traceID, phoneNumber, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappMessageHandler) GetJobStatus(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	jobID := c.Params("job_id")
	ctx := c.Context()

	// Call usecase
	response, err := h.whatsappMessageUsecase.GetJobStatus(ctx, traceID, phoneNumber, jobID)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(response)
}
