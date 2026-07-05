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

const (
	maxTextMessageLen  = 65536
	maxCaptionLen      = 4096
	maxEmojiLen        = 64
	maxLocationName    = 256
	maxLocationAddress = 512
	maxPollQuestion    = 256
	maxPollOptionLen   = 100
	maxPollOptions     = 12
)

func validateLength(field, name string, max int) error {
	if len(field) > max {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("%s exceeds maximum length of %d characters", name, max))
	}
	return nil
}

// validJIDServers are the WhatsApp address spaces a recipient may target.
var validJIDServers = map[string]bool{
	"s.whatsapp.net": true, // individual user
	"g.us":           true, // group
	"lid":            true, // privacy-preserving linked id
	"broadcast":      true, // broadcast / status list
}

// validateRecipient trims and normalizes a recipient identifier into a JID.
// A bare phone number (optionally with +, spaces or dashes) becomes
// "<digits>@s.whatsapp.net"; a value already in JID form must use a known
// server. Empty or otherwise invalid input returns a 400-mapped domain error
// instead of letting whatsmeow fail later with a confusing 500
// ("can't send message to unknown server").
func validateRecipient(msisdn string) (string, error) {
	v := strings.TrimSpace(msisdn)
	if v == "" {
		return "", errDomain.NewError(errDomain.ErrBadRequest,
			errors.New("recipient (msisdn) is required"))
	}

	if user, server, found := strings.Cut(v, "@"); found {
		if user == "" || !validJIDServers[server] {
			return "", errDomain.NewError(errDomain.ErrBadRequest,
				fmt.Errorf("invalid recipient %q", msisdn))
		}
		return v, nil
	}

	// Bare number: keep digits only, then attach the individual-user server.
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.TrimPrefix(v, "+"))
	if digits == "" {
		return "", errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("invalid recipient %q", msisdn))
	}
	return digits + "@s.whatsapp.net", nil
}

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
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Message, "message", maxTextMessageLen); err != nil {
		return nil, nil, err
	}

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
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.String("job_id", jobID),
				customLog.Error(err),
			)
		} else {
			// Successfully queued
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil,
					customLog.String("job_id", jobID),
					customLog.Error(err),
				)
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
		uc.logger.Error(traceID, "Failed to send text message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", req.Msisdn),
			customLog.Error(err),
		)

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
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Caption, "caption", maxCaptionLen); err != nil {
		return nil, nil, err
	}

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
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.String("job_id", jobID),
				customLog.Error(err),
			)
		} else {
			// Successfully queued
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil,
					customLog.String("job_id", jobID),
					customLog.Error(err),
				)
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
		uc.logger.Error(traceID, "Failed to send image message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", req.Msisdn),
			customLog.Error(err),
		)

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

// SendAudioMessage sends an audio message (queued or direct). IsPTT=true
// renders a voice-note bubble.
func (uc *WhatsappMessageUsecase) SendAudioMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendAudioMessageRequest,
	fileHeader *multipart.FileHeader,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	file, err := fileHeader.Open()
	if err != nil {
		uc.logger.Error(traceID, "Failed to open audio file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	defer file.Close()

	audioBytes, err := io.ReadAll(file)
	if err != nil {
		uc.logger.Error(traceID, "Failed to read audio file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	// Sniff the mimetype; for voice notes (PTT) we require opus/ogg, which
	// DetectContentType cannot identify — in that case pass "" and let the
	// client default to "audio/ogg; codecs=opus".
	mimeType := http.DetectContentType(audioBytes)
	if req.IsPTT && !strings.HasPrefix(mimeType, "audio/") {
		mimeType = ""
	}

	if uc.queue != nil && uc.queue.IsHealthy() {
		jobID := uuid.New().String()
		job := domainQueue.OutgoingMessageJob{
			TraceID:     traceID,
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "audio",
			To:          req.Msisdn,
			ImageData:   base64.StdEncoding.EncodeToString(audioBytes),
			MimeType:    mimeType,
			IsPTT:       req.IsPTT,
			IsViewOnce:  req.IsViewOnce,
			CreatedAt:   time.Now().Unix(),
		}

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.String("job_id", jobID),
				customLog.Error(err),
			)
		} else {
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.String("job_id", jobID), customLog.Error(err))
			}
			uc.sendQueuedWebhook(ctx, traceID, job)
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID}, nil
		}
	}

	messageID, err := uc.whatsappManager.SendAudioMessage(ctx, traceID, phoneNumber, req.Msisdn, audioBytes, mimeType, req.IsPTT, req.IsViewOnce)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send audio message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", req.Msisdn),
			customLog.Error(err),
		)
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())
		return nil, nil, err
	}

	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID}, nil, nil
}

// SendVideoMessage sends a video message (queued or direct).
func (uc *WhatsappMessageUsecase) SendVideoMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendVideoMessageRequest,
	fileHeader *multipart.FileHeader,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Caption, "caption", maxCaptionLen); err != nil {
		return nil, nil, err
	}

	file, err := fileHeader.Open()
	if err != nil {
		uc.logger.Error(traceID, "Failed to open video file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	defer file.Close()

	videoBytes, err := io.ReadAll(file)
	if err != nil {
		uc.logger.Error(traceID, "Failed to read video file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	mimeType := http.DetectContentType(videoBytes)

	if uc.queue != nil && uc.queue.IsHealthy() {
		jobID := uuid.New().String()
		job := domainQueue.OutgoingMessageJob{
			TraceID:     traceID,
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "video",
			To:          req.Msisdn,
			ImageData:   base64.StdEncoding.EncodeToString(videoBytes),
			MimeType:    mimeType,
			Caption:     req.Caption,
			IsGif:       req.IsGif,
			IsViewOnce:  req.IsViewOnce,
			CreatedAt:   time.Now().Unix(),
		}

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.String("job_id", jobID),
				customLog.Error(err),
			)
		} else {
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.String("job_id", jobID), customLog.Error(err))
			}
			uc.sendQueuedWebhook(ctx, traceID, job)
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID}, nil
		}
	}

	messageID, err := uc.whatsappManager.SendVideoMessage(ctx, traceID, phoneNumber, req.Msisdn, videoBytes, mimeType, req.Caption, req.IsGif, req.IsViewOnce)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send video message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", req.Msisdn),
			customLog.Error(err),
		)
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())
		return nil, nil, err
	}

	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID}, nil, nil
}

// SendLocationMessage sends a location message (queued or direct)
func (uc *WhatsappMessageUsecase) SendLocationMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendLocationMessageRequest,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Name, "name", maxLocationName); err != nil {
		return nil, nil, err
	}
	if err := validateLength(req.Address, "address", maxLocationAddress); err != nil {
		return nil, nil, err
	}

	if uc.queue != nil && uc.queue.IsHealthy() {
		jobID := uuid.New().String()
		job := domainQueue.OutgoingMessageJob{
			TraceID:         traceID,
			JobID:           jobID,
			PhoneNumber:     phoneNumber,
			Type:            "location",
			To:              req.Msisdn,
			Latitude:        req.Latitude,
			Longitude:       req.Longitude,
			LocationName:    req.Name,
			LocationAddress: req.Address,
			CreatedAt:       time.Now().Unix(),
		}

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.Error(err),
			)
		} else {
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.String("job_id", jobID), customLog.Error(err))
			}
			uc.sendQueuedWebhook(ctx, traceID, job)
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID}, nil
		}
	}

	res, err := uc.limiter.Allow(ctx, phoneNumber)
	if err != nil {
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	if !res.Allowed {
		return nil, nil, errDomain.NewError(errDomain.ErrTooManyRequests, fmt.Errorf("rate limit exceeded, retry after %.0f seconds", res.RetryAfter.Seconds()))
	}

	messageID, err := uc.whatsappManager.SendLocationMessage(ctx, traceID, phoneNumber, req.Msisdn, req.Latitude, req.Longitude, req.Name, req.Address)
	if err != nil {
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())
		return nil, nil, err
	}

	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID}, nil, nil
}

// SendPollMessage sends a poll message (queued or direct)
func (uc *WhatsappMessageUsecase) SendPollMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendPollMessageRequest,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Question, "question", maxPollQuestion); err != nil {
		return nil, nil, err
	}
	if len(req.Options) < 2 {
		return nil, nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("poll requires at least 2 options"))
	}
	if len(req.Options) > maxPollOptions {
		return nil, nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("poll cannot have more than %d options", maxPollOptions))
	}
	for _, opt := range req.Options {
		if err := validateLength(opt, "option", maxPollOptionLen); err != nil {
			return nil, nil, err
		}
	}

	if uc.queue != nil && uc.queue.IsHealthy() {
		jobID := uuid.New().String()
		job := domainQueue.OutgoingMessageJob{
			TraceID:         traceID,
			JobID:           jobID,
			PhoneNumber:     phoneNumber,
			Type:            "poll",
			To:              req.Msisdn,
			Question:        req.Question,
			Options:         req.Options,
			SelectableCount: req.SelectableCount,
			CreatedAt:       time.Now().Unix(),
		}

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.Error(err),
			)
		} else {
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.String("job_id", jobID), customLog.Error(err))
			}
			uc.sendQueuedWebhook(ctx, traceID, job)
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID}, nil
		}
	}

	res, err := uc.limiter.Allow(ctx, phoneNumber)
	if err != nil {
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	if !res.Allowed {
		return nil, nil, errDomain.NewError(errDomain.ErrTooManyRequests, fmt.Errorf("rate limit exceeded, retry after %.0f seconds", res.RetryAfter.Seconds()))
	}

	messageID, err := uc.whatsappManager.SendPollMessage(ctx, traceID, phoneNumber, req.Msisdn, req.Question, req.Options, req.SelectableCount)
	if err != nil {
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())
		return nil, nil, err
	}

	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID}, nil, nil
}

// SendStickerMessage sends a sticker message (queued or direct)
func (uc *WhatsappMessageUsecase) SendStickerMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendStickerMessageRequest,
	fileHeader *multipart.FileHeader,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := validateRecipient(req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("failed to open sticker file: %w", err))
	}
	defer file.Close()

	stickerBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("failed to read sticker file: %w", err))
	}

	mimeType := http.DetectContentType(stickerBytes)

	if uc.queue != nil && uc.queue.IsHealthy() {
		jobID := uuid.New().String()
		job := domainQueue.OutgoingMessageJob{
			TraceID:     traceID,
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "sticker",
			To:          req.Msisdn,
			ImageData:   base64.StdEncoding.EncodeToString(stickerBytes),
			MimeType:    mimeType,
			CreatedAt:   time.Now().Unix(),
		}

		if err := uc.queue.PublishOutgoingMessage(ctx, job); err != nil {
			uc.logger.Warn(traceID, "Queue publish failed, using direct send", nil,
				customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
				customLog.Error(err),
			)
		} else {
			if err := uc.jobRepo.Create(ctx, jobID, "queued", phoneNumber); err != nil {
				uc.logger.Error(traceID, "Failed to create job record", nil, customLog.String("job_id", jobID), customLog.Error(err))
			}
			uc.sendQueuedWebhook(ctx, traceID, job)
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID}, nil
		}
	}

	res, err := uc.limiter.Allow(ctx, phoneNumber)
	if err != nil {
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	if !res.Allowed {
		return nil, nil, errDomain.NewError(errDomain.ErrTooManyRequests, fmt.Errorf("rate limit exceeded, retry after %.0f seconds", res.RetryAfter.Seconds()))
	}

	messageID, err := uc.whatsappManager.SendStickerMessage(ctx, traceID, phoneNumber, req.Msisdn, stickerBytes, mimeType)
	if err != nil {
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())
		return nil, nil, err
	}

	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID}, nil, nil
}

// ReactToMessage reacts to a message
func (uc *WhatsappMessageUsecase) ReactToMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.MessageReactionRequest,
) error {
	to, rErr := validateRecipient(req.Msisdn)
	if rErr != nil {
		return rErr
	}
	req.Msisdn = to

	// Sender is optional: omit it when reacting to your own outgoing message.
	var senderJID string
	if strings.TrimSpace(req.SenderMsisdn) != "" {
		senderJID, rErr = validateRecipient(req.SenderMsisdn)
		if rErr != nil {
			return rErr
		}
	}

	if err := validateLength(req.Emoji, "emoji", maxEmojiLen); err != nil {
		return err
	}

	err := uc.whatsappManager.ReactToMessage(ctx, traceID, phoneNumber, req.Msisdn, senderJID, req.MessageID, req.Emoji)
	if err != nil {
		uc.logger.Error(traceID, "Failed to react to message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", req.MessageID),
			customLog.Error(err),
		)
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
	to, rErr := validateRecipient(req.Msisdn)
	if rErr != nil {
		return rErr
	}
	req.Msisdn = to

	err := uc.whatsappManager.DeleteMessage(ctx, traceID, phoneNumber, req.Msisdn, req.MessageID)
	if err != nil {
		uc.logger.Error(traceID, "Failed to delete message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", req.MessageID),
			customLog.Error(err),
		)
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
	to, rErr := validateRecipient(req.Msisdn)
	if rErr != nil {
		return rErr
	}
	req.Msisdn = to

	if err := validateLength(req.NewMessage, "new_message", maxTextMessageLen); err != nil {
		return err
	}

	err := uc.whatsappManager.EditMessage(ctx, traceID, phoneNumber, req.Msisdn, req.MessageID, req.NewMessage)
	if err != nil {
		uc.logger.Error(traceID, "Failed to edit message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", req.MessageID),
			customLog.Error(err),
		)
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
		uc.logger.Error(traceID, "Failed to get job status", nil,
			customLog.String("job_id", jobID),
			customLog.Error(err),
		)
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if job == nil {
		uc.logger.Error(traceID, "Job not found", nil,
			customLog.String("job_id", jobID),
		)
		return nil, errDomain.NewError(errDomain.ErrNotFound, nil)
	}

	// Verify job belongs to this phone number
	if job.PhoneNumber != phoneNumber {
		uc.logger.Error(traceID, "Job does not belong to this phone number", nil,
			customLog.String("job_id", jobID),
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		)
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
