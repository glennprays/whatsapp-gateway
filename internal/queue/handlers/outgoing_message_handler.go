package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

type OutgoingMessageHandler struct {
	Manager    whatsapp.Manager
	Logger     *customLog.Logger
	JobRepo    *queue.JobRepository
	Repository whatsapp.WhatsAppRepository
	Sender     *whatsapp.WebhookSender
	Config     *config.Config
	Limiter    ratelimiter.Limiter
}

func (h *OutgoingMessageHandler) Handle(ctx context.Context, body []byte, headers amqp.Table) error {
	// Unmarshal outgoing message job
	var err error
	var job domainQueue.OutgoingMessageJob
	if err = json.Unmarshal(body, &job); err != nil {
		return fmt.Errorf("failed to unmarshal outgoing message job: %w", err)
	}

	// Extract trace ID from job, with fallback
	traceID := job.TraceID
	if traceID == "" {
		// Generate fallback trace ID first, then use it for logging
		traceID = fmt.Sprintf("job-%s", job.JobID)
		h.Logger.Warn(traceID, "Job missing trace_id, using fallback", nil,
			customLog.String("job_id", job.JobID),
		)
	}

	masked := whatsapp.MaskedPhoneNumber(job.PhoneNumber)

	var res ratelimiter.Result
	res, err = h.Limiter.Allow(ctx, job.PhoneNumber)
	if err != nil {
		h.Logger.Error(traceID, "Rate limiter error", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return err
	}

	if !res.Allowed {
		errMsg := fmt.Sprintf("Rate limit exceeded. Retry after %s", res.RetryAfter.String())
		h.Logger.Warn(traceID, errMsg, map[string]interface{}{
			"phone_number": job.PhoneNumber,
			"limit":        res.Limit,
			"remaining":    res.Remaining,
		})

		rateLimitErr := &ratelimiter.RateLimitError{}
		return rateLimitErr.BuildError(res)
	}

	// Update job status to processing
	if err := h.JobRepo.UpdateStatus(ctx, job.JobID, "processing", "", ""); err != nil {
		h.Logger.Error(traceID, "Failed to update job status to processing", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
	}

	// Process based on job type using Manager interface
	var messageID string

	switch job.Type {
	case "text":
		messageID, err = h.Manager.SendTextMessage(ctx, traceID, job.PhoneNumber, job.To, job.Text)
	case "image":
		// Decode base64 image data
		imageBytes, decodeErr := base64.StdEncoding.DecodeString(job.ImageData)
		if decodeErr != nil {
			err = fmt.Errorf("failed to decode image data: %w", decodeErr)
		} else {
			messageID, err = h.Manager.SendImageMessage(ctx, traceID, job.PhoneNumber, job.To, imageBytes, job.MimeType, job.Caption, job.IsViewOnce)
		}
	case "react":
		err = h.Manager.ReactToMessage(ctx, traceID, job.PhoneNumber, job.To, job.MessageID, job.Emoji)
		if err == nil {
			messageID = job.MessageID // For react, use the original message ID
		}
	case "delete":
		err = h.Manager.DeleteMessage(ctx, traceID, job.PhoneNumber, job.To, job.MessageID)
		if err == nil {
			messageID = job.MessageID
		}
	case "edit":
		err = h.Manager.EditMessage(ctx, traceID, job.PhoneNumber, job.To, job.MessageID, job.NewText)
		if err == nil {
			messageID = job.MessageID
		}
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	if err != nil {
		errMsg := fmt.Sprintf("Failed to process job: %v", err)
		h.JobRepo.UpdateStatus(ctx, job.JobID, "failed", "", errMsg)

		// Send failure webhook notification
		h.sendStatusWebhook(ctx, traceID, job, domainQueue.EventMessageFailed, "", errMsg)

		return err
	}

	// Update job status to completed
	if err := h.JobRepo.UpdateStatus(ctx, job.JobID, "completed", messageID, ""); err != nil {
		h.Logger.Error(traceID, "Failed to update job status to completed", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.String("message_id", messageID),
			customLog.Error(err),
		)
	}

	// Send success webhook notification
	h.sendStatusWebhook(ctx, traceID, job, domainQueue.EventMessageSent, messageID, "")

	h.Logger.Debug(traceID, "Successfully processed job", nil,
		customLog.String("job_id", job.JobID),
		customLog.String("phone_number", masked),
		customLog.String("message_id", messageID),
		customLog.String("type", job.Type),
	)
	return nil
}

// sendStatusWebhook sends a webhook notification for message status updates
func (h *OutgoingMessageHandler) sendStatusWebhook(
	ctx context.Context,
	traceID string,
	job domainQueue.OutgoingMessageJob,
	event domainQueue.StatusWebhookEvent,
	messageID string,
	errorMsg string,
) {
	// Check if status webhooks are enabled
	if !h.Config.WebhookStatusEventsEnabled {
		return
	}

	// Check if this event is in the enabled events list
	enabledEvents := strings.Split(h.Config.WebhookStatusEvents, ",")
	eventEnabled := false
	for _, e := range enabledEvents {
		if strings.TrimSpace(e) == string(event) {
			eventEnabled = true
			break
		}
	}
	if !eventEnabled {
		h.Logger.Debug(traceID, "Status webhook event not enabled", nil,
			customLog.String("event", string(event)),
			customLog.String("job_id", job.JobID),
		)
		return
	}

	masked := whatsapp.MaskedPhoneNumber(job.PhoneNumber)

	JID, err := h.Manager.GetJIDFromPhoneNumber(job.PhoneNumber)
	if err != nil {
		h.Logger.Error(traceID, "Failed to get JID from phone number for status notification", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return
	}

	// Get webhook configuration for this phone number
	webhook, err := h.Repository.GetWebhook(ctx, JID)
	if err != nil {
		h.Logger.Error(traceID, "Failed to get webhook config for status notification", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return
	}

	if webhook == nil || webhook.Url == "" {
		h.Logger.Debug(traceID, "No webhook URL configured for phone", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
		)
		return
	}

	// Build status webhook payload
	payload := domainQueue.StatusWebhookPayload{
		Event:       string(event),
		JobID:       job.JobID,
		To:          job.To,
		PhoneNumber: job.PhoneNumber,
		Timestamp:   time.Now().Unix(),
		Metadata: map[string]interface{}{
			"type": job.Type,
		},
	}

	if messageID != "" {
		payload.MessageID = messageID
	}
	if errorMsg != "" {
		payload.Error = errorMsg
	}

	// Convert payload to map for webhook sender
	payloadMap := map[string]interface{}{
		"event":        payload.Event,
		"job_id":       payload.JobID,
		"to":           payload.To,
		"phone_number": payload.PhoneNumber,
		"timestamp":    payload.Timestamp,
		"metadata":     payload.Metadata,
	}
	if payload.MessageID != "" {
		payloadMap["message_id"] = payload.MessageID
	}
	if payload.Error != "" {
		payloadMap["error"] = payload.Error
	}

	// Send webhook (with HMAC signing and retry logic)
	if err := h.Sender.Send(ctx, webhook.Url, webhook.HmacSecret, payloadMap); err != nil {
		h.Logger.Error(traceID, "Failed to send status webhook", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.String("event", string(event)),
			customLog.Error(err),
		)
	} else {
		h.Logger.Debug(traceID, "Successfully sent status webhook", nil,
			customLog.String("job_id", job.JobID),
			customLog.String("phone_number", masked),
			customLog.String("event", string(event)),
		)
	}
}
