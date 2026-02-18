package whatsapp_usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
	"github.com/google/uuid"
)

// WhatsappMessageUsecase handles message operations business logic
type WhatsappMessageUsecase struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
	queue           domainQueue.MessageQueue
	jobRepo         *queue.JobRepository
	whatsappRepo    whatsapp.WhatsAppRepository
	webhookSender   *whatsapp.WebhookSender
	config          *config.Config
	limiter         ratelimiter.Limiter
}

// NewWhatsappMessageUsecase creates a new message usecase
func NewWhatsappMessageUsecase(
	manager whatsapp.Manager,
	logger *customLog.Logger,
	queue domainQueue.MessageQueue,
	jobRepo *queue.JobRepository,
	whatsappRepo whatsapp.WhatsAppRepository,
	webhookSender *whatsapp.WebhookSender,
	cfg *config.Config,
	limiter ratelimiter.Limiter,
) *WhatsappMessageUsecase {
	return &WhatsappMessageUsecase{
		whatsappManager: manager,
		logger:          logger,
		queue:           queue,
		jobRepo:         jobRepo,
		whatsappRepo:    whatsappRepo,
		webhookSender:   webhookSender,
		config:          cfg,
		limiter:         limiter,
	}
}

// SendTextMessage sends a text message (queued or direct)
func (uc *WhatsappMessageUsecase) SendTextMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendTextMessageRequest,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	// Check if queue is enabled and healthy
	if uc.queue != nil && uc.queue.IsHealthy() {
		// Queue mode: enqueue job
		jobID := uuid.New().String()

		job := domainQueue.OutgoingMessageJob{
			TraceID:     traceID,
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "text",
			To:          req.Msisdn,
			Text:        req.Message,
			CreatedAt:   time.Now().Unix(),
		}

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			// Queue failed, fall through to direct send
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", []customLog.Field{
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			}, customLog.Error(err))
		} else {
			// Successfully queued
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.Error(err))
			}

			// Send message.queued webhook
			uc.sendQueuedWebhook(ctx, traceID, job)

			return nil, &waDomain.SendMessageQueuedResponse{
				Success: true,
				Status:  "queued",
				JobID:   jobID,
			}, nil
		}
	}

	// Direct mode (or fallback): immediate send
	res, err := uc.limiter.Allow(ctx, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Rate limiter error", map[string]interface{}{
			"phone_number": whatsapp.MaskedPhoneNumber(phoneNumber),
		}, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !res.Allowed {
		uc.logger.Warn(traceID, "Rate limit exceeded", map[string]interface{}{
			"phone_number": whatsapp.MaskedPhoneNumber(phoneNumber),
			"limit":        res.Limit,
			"retry_after":  res.RetryAfter.Seconds(),
			"reset_after":  res.ResetAfter.Seconds(),
			"remaining":    res.Remaining,
		})
		return nil, nil, errDomain.NewError(errDomain.ErrTooManyRequests, errors.New(fmt.Sprintf("Rate limit exceeded. Retry after %.0f seconds", res.RetryAfter.Seconds())))
	}

	messageID, err := uc.whatsappManager.SendTextMessage(ctx, traceID, phoneNumber, req.Msisdn, req.Message)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send text message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))

		// Send message.failed webhook in direct mode
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())

		return nil, nil, err
	}

	// Send message.sent webhook in direct mode
	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)

	return &waDomain.SendMessageResponse{
		Success:   true,
		MessageID: messageID,
	}, nil, nil
}

// SendImageMessage sends an image message (queued or direct)
func (uc *WhatsappMessageUsecase) SendImageMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendImageMessageRequest,
	fileHeader *multipart.FileHeader,
	isViewOnce bool,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	// Open and read image file
	file, err := fileHeader.Open()
	if err != nil {
		uc.logger.Error(traceID, "Failed to open image file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	defer file.Close()

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		uc.logger.Error(traceID, "Failed to read image file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(imageBytes)

	// Check if queue is enabled and healthy
	if uc.queue != nil && uc.queue.IsHealthy() {
		// Queue mode: enqueue job
		jobID := uuid.New().String()

		job := domainQueue.OutgoingMessageJob{
			TraceID:     traceID,
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

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			// Queue failed, fall through to direct send
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", []customLog.Field{
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			}, customLog.Error(err))
		} else {
			// Successfully queued
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.Error(err))
			}

			// Send message.queued webhook
			uc.sendQueuedWebhook(ctx, traceID, job)

			return nil, &waDomain.SendMessageQueuedResponse{
				Success: true,
				Status:  "queued",
				JobID:   jobID,
			}, nil
		}
	}

	// Direct mode (or fallback): immediate send
	messageID, err := uc.whatsappManager.SendImageMessage(ctx, traceID, phoneNumber, req.Msisdn, imageBytes, mimeType, req.Caption, isViewOnce)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send image message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))

		// Send message.failed webhook in direct mode
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())

		return nil, nil, err
	}

	// Send message.sent webhook in direct mode
	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)

	return &waDomain.SendMessageResponse{
		Success:   true,
		MessageID: messageID,
	}, nil, nil
}

// ReactToMessage reacts to a message
func (uc *WhatsappMessageUsecase) ReactToMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.MessageReactionRequest,
) error {
	err := uc.whatsappManager.ReactToMessage(ctx, traceID, phoneNumber, req.Msisdn, req.MessageID, req.Emoji)
	if err != nil {
		uc.logger.Error(traceID, "Failed to react to message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return err
	}

	return nil
}

// DeleteMessage deletes a message
func (uc *WhatsappMessageUsecase) DeleteMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.MessageDeleteRequest,
) error {
	err := uc.whatsappManager.DeleteMessage(ctx, traceID, phoneNumber, req.Msisdn, req.MessageID)
	if err != nil {
		uc.logger.Error(traceID, "Failed to delete message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return err
	}

	return nil
}

// EditMessage edits a message
func (uc *WhatsappMessageUsecase) EditMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.MessageEditRequest,
) error {
	err := uc.whatsappManager.EditMessage(ctx, traceID, phoneNumber, req.Msisdn, req.MessageID, req.NewMessage)
	if err != nil {
		uc.logger.Error(traceID, "Failed to edit message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return err
	}

	return nil
}

// GetJobStatus retrieves job status
func (uc *WhatsappMessageUsecase) GetJobStatus(
	ctx context.Context,
	traceID, phoneNumber, jobID string,
) (*waDomain.JobStatusResponse, error) {
	if jobID == "" {
		uc.logger.Error(traceID, "Job ID is required", nil)
		return nil, errDomain.NewError(errDomain.ErrBadRequest, nil)
	}

	job, err := uc.jobRepo.Get(ctx, jobID)
	if err != nil {
		uc.logger.Error(traceID, "Failed to get job status", []customLog.Field{
			customLog.String("job_id", jobID),
		}, customLog.Error(err))
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if job == nil {
		uc.logger.Error(traceID, "Job not found", []customLog.Field{
			customLog.String("job_id", jobID),
		})
		return nil, errDomain.NewError(errDomain.ErrNotFound, nil)
	}

	// Verify job belongs to this phone number
	if job.PhoneNumber != phoneNumber {
		uc.logger.Error(traceID, "Job does not belong to this phone number", []customLog.Field{
			customLog.String("job_id", jobID),
		})
		return nil, errDomain.NewError(errDomain.ErrForbidden, nil)
	}

	response := &waDomain.JobStatusResponse{
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

	return response, nil
}

// sendQueuedWebhook sends a message.queued webhook notification
func (uc *WhatsappMessageUsecase) sendQueuedWebhook(
	ctx context.Context,
	traceID string,
	job domainQueue.OutgoingMessageJob,
) {
	// Check if webhook status events enabled
	if !uc.config.WebhookStatusEventsEnabled {
		return
	}

	// Check if message.queued is in enabled events
	enabledEvents := strings.Split(uc.config.WebhookStatusEvents, ",")
	found := false
	for _, evt := range enabledEvents {
		if strings.TrimSpace(evt) == string(domainQueue.EventMessageQueued) {
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Get webhook config
	JID, err := uc.whatsappManager.GetJIDFromPhoneNumber(job.PhoneNumber)
	if err != nil {
		return
	}
	webhook, err := uc.whatsappRepo.GetWebhook(ctx, JID)
	if err != nil || webhook == nil || webhook.Url == "" {
		return
	}

	// Build payload
	payload := map[string]interface{}{
		"event":        string(domainQueue.EventMessageQueued),
		"job_id":       job.JobID,
		"to":           job.To,
		"phone_number": job.PhoneNumber,
		"timestamp":    time.Now().Unix(),
	}

	// Send webhook
	if err := uc.webhookSender.Send(ctx, webhook.Url, webhook.HmacSecret, payload); err != nil {
		uc.logger.Error(traceID, "Failed to send queued webhook", nil, customLog.Error(err))
	} else {
		uc.logger.Debug(traceID, fmt.Sprintf("Sent message.queued webhook for job %s", job.JobID), nil)
	}
}

// sendDirectSentWebhook sends a message.sent webhook in direct mode
func (uc *WhatsappMessageUsecase) sendDirectSentWebhook(
	ctx context.Context,
	traceID, phoneNumber, to, messageID string,
) {
	if !uc.config.WebhookStatusEventsEnabled {
		return
	}

	enabledEvents := strings.Split(uc.config.WebhookStatusEvents, ",")
	found := false
	for _, evt := range enabledEvents {
		if strings.TrimSpace(evt) == string(domainQueue.EventMessageSent) {
			found = true
			break
		}
	}
	if !found {
		return
	}

	JID, err := uc.whatsappManager.GetJIDFromPhoneNumber(phoneNumber)
	if err != nil {
		return
	}
	webhook, err := uc.whatsappRepo.GetWebhook(ctx, JID)
	if err != nil || webhook == nil || webhook.Url == "" {
		return
	}

	payload := map[string]interface{}{
		"event":        string(domainQueue.EventMessageSent),
		"to":           to,
		"phone_number": phoneNumber,
		"timestamp":    time.Now().Unix(),
		"message_id":   messageID,
	}

	if err := uc.webhookSender.Send(ctx, webhook.Url, webhook.HmacSecret, payload); err != nil {
		uc.logger.Error(traceID, "Failed to send direct sent webhook", nil, customLog.Error(err))
	} else {
		uc.logger.Debug(traceID, "Sent message.sent webhook (direct mode)", nil)
	}
}

// sendDirectFailedWebhook sends a message.failed webhook in direct mode
func (uc *WhatsappMessageUsecase) sendDirectFailedWebhook(
	ctx context.Context,
	traceID, phoneNumber, to, errorMsg string,
) {
	if !uc.config.WebhookStatusEventsEnabled {
		return
	}

	enabledEvents := strings.Split(uc.config.WebhookStatusEvents, ",")
	found := false
	for _, evt := range enabledEvents {
		if strings.TrimSpace(evt) == string(domainQueue.EventMessageFailed) {
			found = true
			break
		}
	}
	if !found {
		return
	}

	JID, err := uc.whatsappManager.GetJIDFromPhoneNumber(phoneNumber)
	if err != nil {
		return
	}
	webhook, err := uc.whatsappRepo.GetWebhook(ctx, JID)
	if err != nil || webhook == nil || webhook.Url == "" {
		return
	}

	payload := map[string]interface{}{
		"event":        string(domainQueue.EventMessageFailed),
		"to":           to,
		"phone_number": phoneNumber,
		"timestamp":    time.Now().Unix(),
		"error":        errorMsg,
	}

	if err := uc.webhookSender.Send(ctx, webhook.Url, webhook.HmacSecret, payload); err != nil {
		uc.logger.Error(traceID, "Failed to send direct failed webhook", nil, customLog.Error(err))
	}
}
