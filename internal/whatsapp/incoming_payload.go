package whatsapp

import (
	"context"
	"time"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// BuildWebhookPayload is the single incoming-message webhook builder shared by
// direct mode (handler.deliverWebhook) and queue mode
// (queue/handlers.IncomingEventHandler). Both modes must emit byte-identical
// payloads for the same event; this function is the authority on the shape,
// including ContactMessage handling, media error fields, nil-message handling
// and JID resolution.
//
// Media is downloaded at most once per event (per call), before any
// subscription fan-out. Each download runs on its own context bounded by
// mediaTimeout (callers pass the configured INCOMING_MEDIA_DOWNLOAD_TIMEOUT_
// SECONDS; <= 0 is normalized to 5s upstream).
func BuildWebhookPayload(msg *events.Message, mediaDownloader MediaDownloader, traceID string, phoneNumber string, client *whatsmeow.Client, mediaTimeout time.Duration) map[string]interface{} {
	from, addrMode := ConvertJIDToNonADLID(msg.Info.Sender, msg.Info.SenderAlt, msg.Info.Chat, client)
	payload := map[string]interface{}{
		"event":           string(domainQueue.EventMessageIncoming),
		"message_id":      msg.Info.ID,
		"timestamp":       msg.Info.Timestamp.Unix(),
		"from":            from,
		"addressing_mode": addrMode,
		"chat":            msg.Info.Chat.String(),
		"is_group":        msg.Info.IsGroup,
		"push_name":       msg.Info.PushName,
	}

	// Extract message content based on type
	if msg.Message == nil {
		return payload
	}

	// Extract media URLs
	var mediaInfo map[string]interface{}

	switch {
	case msg.Message.Conversation != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.Conversation

	case msg.Message.ExtendedTextMessage != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.ExtendedTextMessage.Text

	case msg.Message.ImageMessage != nil:
		payload["type"] = "image"
		whatsappURL := msg.Message.ImageMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.ImageMessage, "image", mediaTimeout)
		mediaInfo = buildImageMediaInfo(msg.Message.ImageMessage, storageURL, whatsappURL, err)

	case msg.Message.VideoMessage != nil:
		payload["type"] = "video"
		whatsappURL := msg.Message.VideoMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.VideoMessage, "video", mediaTimeout)
		mediaInfo = buildVideoMediaInfo(msg.Message.VideoMessage, storageURL, whatsappURL, err)

	case msg.Message.AudioMessage != nil:
		payload["type"] = "audio"
		whatsappURL := msg.Message.AudioMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.AudioMessage, "audio", mediaTimeout)
		mediaInfo = buildAudioMediaInfo(msg.Message.AudioMessage, storageURL, whatsappURL, err)

	case msg.Message.DocumentMessage != nil:
		payload["type"] = "document"
		whatsappURL := msg.Message.DocumentMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.DocumentMessage, "document", mediaTimeout)
		mediaInfo = buildDocumentMediaInfo(msg.Message.DocumentMessage, storageURL, whatsappURL, err)

	case msg.Message.StickerMessage != nil:
		payload["type"] = "sticker"
		whatsappURL := msg.Message.StickerMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.StickerMessage, "sticker", mediaTimeout)
		mediaInfo = buildStickerMediaInfo(msg.Message.StickerMessage, storageURL, whatsappURL, err)

	case msg.Message.ContactMessage != nil:
		payload["type"] = "contact"
		if msg.Message.ContactMessage.DisplayName != nil {
			payload["display_name"] = *msg.Message.ContactMessage.DisplayName
		}

	case msg.Message.LocationMessage != nil:
		payload["type"] = "location"
		if msg.Message.LocationMessage.DegreesLatitude != nil {
			payload["latitude"] = *msg.Message.LocationMessage.DegreesLatitude
		}
		if msg.Message.LocationMessage.DegreesLongitude != nil {
			payload["longitude"] = *msg.Message.LocationMessage.DegreesLongitude
		}
		if name := msg.Message.LocationMessage.GetName(); name != "" {
			payload["name"] = name
		}
		if addr := msg.Message.LocationMessage.GetAddress(); addr != "" {
			payload["address"] = addr
		}

	case msg.Message.PollCreationMessage != nil:
		payload["type"] = "poll"
		payload["question"] = msg.Message.PollCreationMessage.GetName()
		var opts []string
		for _, o := range msg.Message.PollCreationMessage.GetOptions() {
			opts = append(opts, o.GetOptionName())
		}
		payload["options"] = opts
		payload["selectable_count"] = msg.Message.PollCreationMessage.GetSelectableOptionsCount()

	default:
		payload["type"] = "unknown"
	}

	if mediaInfo != nil {
		payload["media"] = mediaInfo
	}

	return payload
}

func downloadMedia(
	mediaDownloader MediaDownloader,
	traceID string,
	phoneNumber string,
	mediaMessage DownloadableMessage,
	mediaType string,
	mediaTimeout time.Duration,
) (string, error) {
	if mediaDownloader == nil {
		return "", nil
	}
	if mediaTimeout <= 0 {
		mediaTimeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), mediaTimeout)
	defer cancel()
	return mediaDownloader.DownloadAndStoreMedia(
		ctx,
		traceID,
		phoneNumber,
		mediaMessage,
		mediaType,
	)
}

func buildImageMediaInfo(
	imgMsg *proto.ImageMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "image",
		"mime_type": imgMsg.GetMimetype(),
		"size":      imgMsg.GetFileLength(),
	}

	// Handle URL based on storage result
	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	if caption := imgMsg.GetCaption(); caption != "" {
		mediaInfo["caption"] = caption
	}

	return mediaInfo
}

func buildVideoMediaInfo(
	vidMsg *proto.VideoMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "video",
		"mime_type": vidMsg.GetMimetype(),
		"size":      vidMsg.GetFileLength(),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	if caption := vidMsg.GetCaption(); caption != "" {
		mediaInfo["caption"] = caption
	}

	return mediaInfo
}

func buildAudioMediaInfo(
	audioMsg *proto.AudioMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "audio",
		"mime_type": audioMsg.GetMimetype(),
		"size":      audioMsg.GetFileLength(),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	return mediaInfo
}

func buildDocumentMediaInfo(
	docMsg *proto.DocumentMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "document",
		"file_name": docMsg.GetFileName(),
		"mime_type": docMsg.GetMimetype(),
		"size":      docMsg.GetFileLength(),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	return mediaInfo
}

func buildStickerMediaInfo(
	stickerMsg *proto.StickerMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type": "sticker",
	}

	if stickerMsg.Mimetype != nil {
		mediaInfo["mime_type"] = *stickerMsg.Mimetype
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	return mediaInfo
}
