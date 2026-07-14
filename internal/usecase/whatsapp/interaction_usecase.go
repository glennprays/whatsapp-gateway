package whatsapp_usecase

import (
	"context"
	"fmt"
	"strings"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

// spendActionBudget applies the interim per-account cap on conversation actions
// (mark-read, typing, react) until roadmap #2 pacing lands. It reuses the
// existing outbound limiter under a distinct `action:` key namespace so actions
// get their own bucket without touching the message budget.
//
// ponytail: interim best-effort cap; replaced by proper outbound pacing (#2).
func (uc *WhatsappMessageUsecase) spendActionBudget(ctx context.Context, phoneNumber string) error {
	res, err := uc.limiter.Allow(ctx, "action:"+phoneNumber)
	if err == nil && !res.Allowed {
		return errDomain.NewError(errDomain.ErrTooManyRequests,
			fmt.Errorf("action rate limit exceeded; retry after %ds", int(res.RetryAfter.Seconds())+1))
	}
	return nil
}

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

	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
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

	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return err
	}
	return uc.whatsappManager.SendChatPresence(ctx, traceID, phoneNumber, target, state, media)
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
