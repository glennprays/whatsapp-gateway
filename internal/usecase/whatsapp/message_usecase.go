package whatsapp_usecase

import (
	"context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	customLog "github.com/glennprays/log"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/google/uuid"
)

// WhatsappMessageUsecase handles message operations business logic
type WhatsappMessageUsecase struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
	queue           domainQueue.MessageQueue
	jobRepo         *queue.JobRepository
}

// NewWhatsappMessageUsecase creates a new message usecase
func NewWhatsappMessageUsecase(
	manager whatsapp.Manager,
	logger *customLog.Logger,
	queue domainQueue.MessageQueue,
	jobRepo *queue.JobRepository,
) *WhatsappMessageUsecase {
	return &WhatsappMessageUsecase{
		whatsappManager: manager,
		logger:          logger,
		queue:           queue,
		jobRepo:         jobRepo,
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

			return nil, &waDomain.SendMessageQueuedResponse{
				Success: true,
				Status:  "queued",
				JobID:   jobID,
			}, nil
		}
	}

	// Direct mode (or fallback): immediate send
	messageID, err := uc.whatsappManager.SendTextMessage(ctx, phoneNumber, req.Msisdn, req.Message)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send text message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return nil, nil, err
	}

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

			return nil, &waDomain.SendMessageQueuedResponse{
				Success: true,
				Status:  "queued",
				JobID:   jobID,
			}, nil
		}
	}

	// Direct mode (or fallback): immediate send
	messageID, err := uc.whatsappManager.SendImageMessage(ctx, phoneNumber, req.Msisdn, imageBytes, mimeType, req.Caption, isViewOnce)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send image message", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return nil, nil, err
	}

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
	err := uc.whatsappManager.ReactToMessage(ctx, phoneNumber, req.Msisdn, req.MessageID, req.Emoji)
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
	err := uc.whatsappManager.DeleteMessage(ctx, phoneNumber, req.Msisdn, req.MessageID)
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
	err := uc.whatsappManager.EditMessage(ctx, phoneNumber, req.Msisdn, req.MessageID, req.NewMessage)
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
