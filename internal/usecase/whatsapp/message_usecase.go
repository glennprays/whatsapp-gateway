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
	"go.mau.fi/whatsmeow/types"
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

// mediaMimeAllowlists caps which MIME types each media kind accepts. Documents
// are intentionally unrestricted (arbitrary file types).
var mediaMimeAllowlists = map[string]map[string]bool{
	"image":   {"image/jpeg": true, "image/png": true, "image/webp": true},
	"sticker": {"image/webp": true},
	"audio":   {"audio/ogg": true, "audio/mpeg": true, "audio/mp4": true, "audio/aac": true, "audio/x-aac": true, "audio/amr": true, "audio/3gpp": true, "audio/3gpp2": true, "audio/webm": true, "audio/x-m4a": true, "application/ogg": true},
	"video":   {"video/mp4": true, "video/3gpp": true, "video/webm": true},
}

// validateMediaSize enforces the outbound size cap. Call before io.ReadAll so
// oversized uploads are rejected without ever being read into memory.
func (uc *WhatsappMessageUsecase) validateMediaSize(size int64) error {
	if uc.config.MaxUploadBytes > 0 && size > uc.config.MaxUploadBytes {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("media exceeds maximum upload size of %d bytes", uc.config.MaxUploadBytes))
	}
	return nil
}

// validateMediaMime enforces a per-kind MIME allow-list. Pass detectedMime=""
// to opt out (e.g. PTT voice notes whose opus/ogg sniffing is unreliable).
func validateMediaMime(kind, detectedMime string) error {
	allow, ok := mediaMimeAllowlists[kind]
	if !ok || detectedMime == "" {
		return nil
	}
	if !allow[detectedMime] {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("unsupported %s mime type %q", kind, detectedMime))
	}
	return nil
}

// validJIDServers are the WhatsApp address spaces a recipient may target.
// `broadcast` is intentionally excluded: whatsmeow does not support sending to
// non-status broadcast lists, so we reject it early (400) instead of letting it
// fail late as a confusing 500.
var validJIDServers = map[string]bool{
	"s.whatsapp.net": true, // individual user
	"g.us":           true, // group
	"lid":            true, // privacy-preserving linked id
}

// resolveChat resolves the canonical recipient JID from a request's `chat`
// field (preferred) or the legacy `msisdn` alias — chat wins when both are set.
// A bare phone number (optionally with +, spaces or dashes) becomes
// "<digits>@s.whatsapp.net". A value already in JID form must use a known
// server (s.whatsapp.net, g.us, lid) with a digits-only user; any device/agent
// suffix is stripped and the server is lowercased to the canonical non-AD form.
// Empty or otherwise invalid input returns a 400-mapped domain error instead of
// letting whatsmeow fail later with a confusing 500.
func resolveChat(chat, msisdn string) (string, error) {
	v := strings.TrimSpace(chat)
	if v == "" {
		v = strings.TrimSpace(msisdn)
	}
	if v == "" {
		return "", errDomain.NewError(errDomain.ErrBadRequest,
			errors.New("chat (or msisdn) is required"))
	}

	if strings.ContainsRune(v, '@') {
		jid, err := types.ParseJID(v)
		if err != nil || !isAllDigits(jid.User) {
			return "", errDomain.NewError(errDomain.ErrBadRequest,
				fmt.Errorf("invalid recipient %q", v))
		}
		server := strings.ToLower(jid.Server)
		if !validJIDServers[server] {
			return "", errDomain.NewError(errDomain.ErrBadRequest,
				fmt.Errorf("invalid recipient %q", v))
		}
		// Canonical non-AD form: user@server with device/agent stripped.
		return jid.User + "@" + server, nil
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
			fmt.Errorf("invalid recipient %q", v))
	}
	return digits + "@s.whatsapp.net", nil
}

// isAllDigits reports whether s is non-empty and contains only ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
	queryCache      *ttlCache           // short-TTL cache for server-hitting reads
	queryBudget     ratelimiter.Limiter // per-account read budget (spent on cache miss)
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
		queryCache:      newTTLCache(time.Duration(cfg.ReadQueryCacheTTLSeconds) * time.Second),
		queryBudget:     newQueryBudget(cfg.ReadQueryBudget, cfg.ReadQueryWindowSeconds),
	}
}

// SendTextMessage sends a text message (queued or direct)
func (uc *WhatsappMessageUsecase) SendTextMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendTextMessageRequest,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Message, "message", maxTextMessageLen); err != nil {
		return nil, nil, err
	}

	msgCtx, err := uc.buildMessageContext(req.ReplyToID, req.ReplyToSender, req.ReplyToText, req.Mentions)
	if err != nil {
		return nil, nil, err
	}

	// Check if queue is enabled and healthy
	if uc.queue != nil && uc.queue.IsHealthy() {
		// Queue mode: enqueue job
		jobID := uuid.New().String()

		job := domainQueue.OutgoingMessageJob{
			TraceID:       traceID,
			JobID:         jobID,
			PhoneNumber:   phoneNumber,
			Type:          "text",
			To:            req.Msisdn,
			Text:          req.Message,
			ReplyToID:     msgCtx.ReplyToID,
			ReplyToSender: msgCtx.ReplyToSender,
			ReplyToText:   msgCtx.ReplyToText,
			Mentions:      msgCtx.Mentions,
			CreatedAt:     time.Now().Unix(),
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
				Chat:    req.Msisdn,
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

	messageID, err := uc.whatsappManager.SendTextMessage(ctx, traceID, phoneNumber, req.Msisdn, req.Message, msgCtx)
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
		Chat:      req.Msisdn,
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
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Caption, "caption", maxCaptionLen); err != nil {
		return nil, nil, err
	}

	// Open and read image file
	if err := uc.validateMediaSize(fileHeader.Size); err != nil {
		return nil, nil, err
	}
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
	if err := validateMediaMime("image", mimeType); err != nil {
		return nil, nil, err
	}
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
				Chat:    req.Msisdn,
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
		Chat:      req.Msisdn,
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
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := uc.validateMediaSize(fileHeader.Size); err != nil {
		return nil, nil, err
	}
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
	if err := validateMediaMime("audio", mimeType); err != nil {
		return nil, nil, err
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
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID, Chat: req.Msisdn}, nil
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
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID, Chat: req.Msisdn}, nil, nil
}

// SendVideoMessage sends a video message (queued or direct).
func (uc *WhatsappMessageUsecase) SendVideoMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendVideoMessageRequest,
	fileHeader *multipart.FileHeader,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Caption, "caption", maxCaptionLen); err != nil {
		return nil, nil, err
	}

	if err := uc.validateMediaSize(fileHeader.Size); err != nil {
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
	if err := validateMediaMime("video", mimeType); err != nil {
		return nil, nil, err
	}

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
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID, Chat: req.Msisdn}, nil
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
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID, Chat: req.Msisdn}, nil, nil
}

// SendDocumentMessage sends a document (file) message (queued or direct).
func (uc *WhatsappMessageUsecase) SendDocumentMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendDocumentMessageRequest,
	fileHeader *multipart.FileHeader,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := validateLength(req.Caption, "caption", maxCaptionLen); err != nil {
		return nil, nil, err
	}

	// Default the visible file name from the upload when the caller omits it.
	if req.FileName == "" {
		req.FileName = fileHeader.Filename
	}
	if req.FileName == "" {
		req.FileName = "file"
	}

	if err := uc.validateMediaSize(fileHeader.Size); err != nil {
		return nil, nil, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		uc.logger.Error(traceID, "Failed to open document file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}
	defer file.Close()

	docBytes, err := io.ReadAll(file)
	if err != nil {
		uc.logger.Error(traceID, "Failed to read document file", nil, customLog.Error(err))
		return nil, nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	mimeType := http.DetectContentType(docBytes)

	if uc.queue != nil && uc.queue.IsHealthy() {
		jobID := uuid.New().String()
		job := domainQueue.OutgoingMessageJob{
			TraceID:     traceID,
			JobID:       jobID,
			PhoneNumber: phoneNumber,
			Type:        "document",
			To:          req.Msisdn,
			ImageData:   base64.StdEncoding.EncodeToString(docBytes),
			MimeType:    mimeType,
			FileName:    req.FileName,
			Caption:     req.Caption,
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
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID, Chat: req.Msisdn}, nil
		}
	}

	messageID, err := uc.whatsappManager.SendDocumentMessage(ctx, traceID, phoneNumber, req.Msisdn, docBytes, mimeType, req.FileName, req.Caption)
	if err != nil {
		uc.logger.Error(traceID, "Failed to send document message", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", req.Msisdn),
			customLog.Error(err),
		)
		uc.sendDirectFailedWebhook(ctx, traceID, phoneNumber, req.Msisdn, err.Error())
		return nil, nil, err
	}

	uc.sendDirectSentWebhook(ctx, traceID, phoneNumber, req.Msisdn, messageID)
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID, Chat: req.Msisdn}, nil, nil
}

// SendLocationMessage sends a location message (queued or direct)
func (uc *WhatsappMessageUsecase) SendLocationMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendLocationMessageRequest,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
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
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID, Chat: req.Msisdn}, nil
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
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID, Chat: req.Msisdn}, nil, nil
}

// SendPollMessage sends a poll message (queued or direct)
func (uc *WhatsappMessageUsecase) SendPollMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendPollMessageRequest,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
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
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID, Chat: req.Msisdn}, nil
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
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID, Chat: req.Msisdn}, nil, nil
}

// SendStickerMessage sends a sticker message (queued or direct)
func (uc *WhatsappMessageUsecase) SendStickerMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SendStickerMessageRequest,
	fileHeader *multipart.FileHeader,
) (*waDomain.SendMessageResponse, *waDomain.SendMessageQueuedResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return nil, nil, err
	}
	req.Msisdn = to

	if err := uc.validateMediaSize(fileHeader.Size); err != nil {
		return nil, nil, err
	}
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
	if err := validateMediaMime("sticker", mimeType); err != nil {
		return nil, nil, err
	}

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
			return nil, &waDomain.SendMessageQueuedResponse{Success: true, Status: "queued", JobID: jobID, Chat: req.Msisdn}, nil
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
	return &waDomain.SendMessageResponse{Success: true, MessageID: messageID, Chat: req.Msisdn}, nil, nil
}

// ReactToMessage reacts to a message
func (uc *WhatsappMessageUsecase) ReactToMessage(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.MessageReactionRequest,
) error {
	to, rErr := resolveChat(req.Chat, req.Msisdn)
	if rErr != nil {
		return rErr
	}
	req.Msisdn = to

	// Sender is optional: omit it when reacting to your own outgoing message.
	var senderJID string
	if strings.TrimSpace(req.SenderMsisdn) != "" {
		senderJID, rErr = resolveChat("", req.SenderMsisdn)
		if rErr != nil {
			return rErr
		}
	}

	if err := validateLength(req.Emoji, "emoji", maxEmojiLen); err != nil {
		return err
	}

	// Reactions are conversation actions — counted against the interim action cap.
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
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
	to, rErr := resolveChat(req.Chat, req.Msisdn)
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
	to, rErr := resolveChat(req.Chat, req.Msisdn)
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
// CheckNumber validates whether a recipient is registered on WhatsApp before
// sending. Also warms the LID<->PN cache, reducing send fragility.
func (uc *WhatsappMessageUsecase) CheckNumber(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.ContactCheckRequest,
) (waDomain.ContactCheckResponse, error) {
	to, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return waDomain.ContactCheckResponse{}, err
	}

	// IsOnWhatsApp expects a bare phone number; strip the JID server if present.
	query := to
	if user, _, ok := strings.Cut(query, "@"); ok {
		query = user
	}

	resp, err := uc.whatsappManager.CheckNumber(ctx, traceID, phoneNumber, query)
	if err != nil {
		uc.logger.Error(traceID, "Failed to check number", nil,
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
			customLog.String("query", query),
			customLog.Error(err),
		)
		return waDomain.ContactCheckResponse{}, err
	}
	return resp, nil
}

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
