package whatsapp_usecase

import (
	"context"
	"fmt"
	"strings"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

// MarkRead marks messages in a chat as read (blue ticks). Group chats require
// the message author in req.Sender. Counts against the interim action cap.
func (uc *WhatsappMessageUsecase) MarkRead(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.MarkReadRequest,
) error {
	target, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return err
	}
	if len(req.MessageIDs) == 0 {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("message_ids is required"))
	}
	if strings.HasSuffix(target, "@"+groupServerSuffix) && strings.TrimSpace(req.Sender) == "" {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("sender is required for group chats"))
	}

	var sender string
	if strings.TrimSpace(req.Sender) != "" {
		sender, err = resolveChat(req.Sender, "")
		if err != nil {
			return err
		}
	}

	if err := uc.pace(ctx, phoneNumber, target, 1); err != nil {
		return err
	}
	return uc.whatsappManager.MarkRead(ctx, traceID, phoneNumber, target, sender, req.MessageIDs)
}

// SendChatPresence sets the typing indicator in a chat. Counts against the
// interim action cap. The indicator auto-expires client-side after a few
// seconds, so an explicit "paused" is optional.
func (uc *WhatsappMessageUsecase) SendChatPresence(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.ChatPresenceRequest,
) error {
	target, err := resolveChat(req.Chat, req.Msisdn)
	if err != nil {
		return err
	}
	state, media, err := resolvePresenceState(req.State)
	if err != nil {
		return err
	}

	if err := uc.pace(ctx, phoneNumber, target, 1); err != nil {
		return err
	}
	return uc.whatsappManager.SendChatPresence(ctx, traceID, phoneNumber, target, state, media)
}

// buildMessageContext resolves the caller-supplied reply + mentions metadata
// into canonical JIDs. Returns a non-nil (possibly empty) context so callers can
// pass it straight through. reply_to_sender and each mention are resolved via
// resolveChat (number → user JID); an invalid one is a 400.
func (uc *WhatsappMessageUsecase) buildMessageContext(replyToID, replyToSender, replyToText string, mentions []string) (*waDomain.MessageContext, error) {
	mc := &waDomain.MessageContext{
		ReplyToID:   strings.TrimSpace(replyToID),
		ReplyToText: replyToText,
	}
	if s := strings.TrimSpace(replyToSender); s != "" {
		jid, err := resolveChat(s, "")
		if err != nil {
			return nil, err
		}
		mc.ReplyToSender = jid
	}
	for _, m := range mentions {
		if strings.TrimSpace(m) == "" {
			continue
		}
		jid, err := resolveChat(m, "")
		if err != nil {
			return nil, err
		}
		mc.Mentions = append(mc.Mentions, jid)
	}
	return mc, nil
}

// resolvePresenceState maps the API's state to whatsmeow's (ChatPresence,
// ChatPresenceMedia). "recording" is composing with audio media.
func resolvePresenceState(s string) (state, media string, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "composing", "typing":
		return "composing", "", nil
	case "recording":
		return "composing", "audio", nil
	case "paused", "stop":
		return "paused", "", nil
	default:
		return "", "", errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("invalid presence state %q (want composing, recording, or paused)", s))
	}
}
