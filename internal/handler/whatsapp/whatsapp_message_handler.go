package whatsapp_handler

import (
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"time"

	customLog "github.com/glennprays/log"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type WhatsappMessageHandler struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
	queue           domainQueue.MessageQueue
	jobRepo         *queue.JobRepository
}

func NewWhatsappMessageHandler(manager whatsapp.Manager, logger *customLog.Logger, queue domainQueue.MessageQueue, jobRepo *queue.JobRepository) *WhatsappMessageHandler {
	return &WhatsappMessageHandler{
		whatsappManager: manager,
		logger:          logger,
		queue:           queue,
		jobRepo:         jobRepo,
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

	// Check if queue is enabled and healthy
	if h.queue != nil && h.queue.IsHealthy() {
		// Queue mode: enqueue job
		jobID := uuid.New().String()

		job := domainQueue.OutgoingMessageJob{
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "text",
			To:          req.Msisdn,
			Text:        req.Message,
			CreatedAt:   time.Now().Unix(),
		}

		if err := h.queue.PublishOutgoingMessage(ctx, job); err != nil {
			// Queue failed, fall through to direct send
			h.logger.Warn(traceID, "Queue publish failed, using direct send", []customLog.Field{
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			}, customLog.Error(err))
		} else {
			// Successfully queued
			if err := h.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				h.logger.Error(traceID, "Failed to create job record", nil, customLog.Error(err))
			}

			return c.Status(http.StatusAccepted).JSON(waDomain.SendMessageQueuedResponse{
				Success: true,
				Status:  "queued",
				JobID:   jobID,
			})
		}
	}

	// Direct mode (or fallback): immediate send
	messageID, err := h.whatsappManager.SendTextMessage(ctx, phoneNumber, req.Msisdn, req.Message)
	if err != nil {
		h.logger.Error(traceID, "Failed to send text message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(waDomain.SendMessageResponse{
		Success:   true,
		MessageID: messageID,
	})
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

	// Open and read image file
	file, err := fileHeader.Open()
	if err != nil {
		h.logger.Error(traceID, "Failed to open image file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}
	defer file.Close()

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error(traceID, "Failed to read image file", nil, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(imageBytes)

	// Parse is_view_once
	isViewOnceStr := c.FormValue("is_view_once")
	isViewOnce := false
	if isViewOnceStr != "" {
		isViewOnce, _ = strconv.ParseBool(isViewOnceStr)
	}

	ctx := c.Context()

	// Check if queue is enabled and healthy
	if h.queue != nil && h.queue.IsHealthy() {
		// Queue mode: enqueue job
		jobID := uuid.New().String()

		job := domainQueue.OutgoingMessageJob{
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "image",
			To:          req.Msisdn,
			ImageData:   base64.StdEncoding.EncodeToString(imageBytes),
			MimeType:    mimeType,
			Caption:     req.Caption,
			IsViewOnce:  isViewOnce,
			CreatedAt:   time.Now().Unix(),
		}

		if err := h.queue.PublishOutgoingMessage(ctx, job); err != nil {
			// Queue failed, fall through to direct send
			h.logger.Warn(traceID, "Queue publish failed, using direct send", []customLog.Field{
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			}, customLog.Error(err))
		} else {
			// Successfully queued
			if err := h.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				h.logger.Error(traceID, "Failed to create job record", nil, customLog.Error(err))
			}

			return c.Status(http.StatusAccepted).JSON(waDomain.SendMessageQueuedResponse{
				Success: true,
				Status:  "queued",
				JobID:   jobID,
			})
		}
	}

	// Direct mode (or fallback): immediate send
	messageID, err := h.whatsappManager.SendImageMessage(ctx, phoneNumber, req.Msisdn, imageBytes, mimeType, req.Caption, isViewOnce)
	if err != nil {
		h.logger.Error(traceID, "Failed to send image message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(waDomain.SendMessageResponse{
		Success:   true,
		MessageID: messageID,
	})
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
	err := h.whatsappManager.ReactToMessage(ctx, phoneNumber, req.Msisdn, req.MessageID, req.Emoji)
	if err != nil {
		h.logger.Error(traceID, "Failed to react to message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
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
	err := h.whatsappManager.DeleteMessage(ctx, phoneNumber, req.Msisdn, req.MessageID)
	if err != nil {
		h.logger.Error(traceID, "Failed to delete message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
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
	err := h.whatsappManager.EditMessage(ctx, phoneNumber, req.Msisdn, req.MessageID, req.NewMessage)
	if err != nil {
		h.logger.Error(traceID, "Failed to edit message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(waDomain.MessageOperationResponse{
		Success: true,
	})
}

func (h *WhatsappMessageHandler) GetJobStatus(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	jobID := c.Params("job_id")
	if jobID == "" {
		h.logger.Error(traceID, "Job ID is required", nil)
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrBadRequest, nil))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	ctx := c.Context()
	job, err := h.jobRepo.Get(ctx, jobID)
	if err != nil {
		h.logger.Error(traceID, "Failed to get job status", []customLog.Field{
			customLog.String("job_id", jobID),
		}, customLog.Error(err))
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if job == nil {
		h.logger.Error(traceID, "Job not found", []customLog.Field{
			customLog.String("job_id", jobID),
		})
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrNotFound, nil))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Verify job belongs to this phone number
	if job.PhoneNumber != phoneNumber {
		h.logger.Error(traceID, "Job does not belong to this phone number", []customLog.Field{
			customLog.String("job_id", jobID),
		})
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrForbidden, nil))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	response := waDomain.JobStatusResponse{
		JobID:     job.JobID,
		Status:    job.Status,
		MessageID: job.MessageID,
		Error:     job.ErrorMessage,
		CreatedAt: job.CreatedAt.Format(time.RFC3339),
	}

	if job.CompletedAt != nil {
		completedAt := job.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedAt
	}

	return c.Status(http.StatusOK).JSON(response)
}
