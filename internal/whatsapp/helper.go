package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"runtime"
	"strings"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func WhatsappDecomposeJID(jid string) string {
	phoneNumber := jid
	if strings.ContainsRune(jid, '@') {
		buffers := strings.Split(jid, "@")
		phoneNumber = buffers[0]
	}

	if phoneNumber[0] == '+' {
		phoneNumber = phoneNumber[1:]
	}

	return phoneNumber
}

// StripDeviceIDFromJID removes the device ID suffix from a WhatsApp JID
// Example: "6281910481554:2@s.whatsapp.net" → "6281910481554@s.whatsapp.net"
// Example: "6281910481554@s.whatsapp.net" → "6281910481554@s.whatsapp.net" (no change)
func StripDeviceIDFromJID(jid string) string {
	// Find the @ symbol position
	atIdx := strings.Index(jid, "@")
	if atIdx == -1 {
		return jid // Invalid JID format, return as-is
	}

	// Find the first colon before the @ symbol
	colonIdx := strings.Index(jid[:atIdx], ":")
	if colonIdx == -1 {
		return jid // No device ID suffix, return as-is
	}

	// Remove the device ID suffix (everything from : to @)
	return jid[:colonIdx] + jid[atIdx:]
}

func MaskedPhoneNumber(phoneNumber string) string {
	if len(phoneNumber) < 4 {
		return phoneNumber
	}
	return phoneNumber[:len(phoneNumber)-4] + "xxxx"
}

func WhatsAppGetUserOS() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	default:
		return "Linux"
	}
}

func WhatsappGenerateQRCode(ctx context.Context, qrChan <-chan whatsmeow.QRChannelItem) (string, int, error) {
	for {
		select {

		case evt, ok := <-qrChan:
			if !ok {
				return "", 0, errors.New("qr channel closed")
			}

			if evt.Error != nil {
				return "", 0, evt.Error
			}

			if evt.Event == "code" {

				qrPng, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
				if err != nil {
					return "", 0, err
				}

				return base64.StdEncoding.EncodeToString(qrPng), int(evt.Timeout.Seconds()), nil
			}
		case <-ctx.Done():
			return "", 0, errors.New("context cancelled while waiting for QR code")
		}
	}
}

// ConvertJIDToNonADLID resolves a sender JID to a phone-number (@s.whatsapp.net)
// form for webhook/buffer payloads. It returns the resolved address plus an
// addressing mode: "pn" (a callable @s.whatsapp.net address) or "lid" (the
// only available identifier was an opaque @lid — consumers should not assume
// `from` is dialable).
//
// Resolution order for @lid senders:
//  1. msg.Info.SenderAlt — whatsmeow fills this with the sender's PN JID on
//     every LID-addressed message; strictly more reliable than a DB lookup.
//  2. client.Store.LIDs.GetPNForLID — persisted LID→PN mapping.
//  3. chat JID — only useful for 1:1 chats where chat == peer PN.
//  4. raw sender @lid — last resort (group LID-only participant); mode "lid".
func ConvertJIDToNonADLID(senderJID types.JID, senderAlt types.JID, chatJID types.JID, client *whatsmeow.Client) (string, string) {
	const pnSuffix = "@s.whatsapp.net"
	senderStr := senderJID.String()

	// Already a phone-number JID.
	if strings.HasSuffix(senderStr, pnSuffix) {
		return StripDeviceIDFromJID(senderStr), "pn"
	}

	if strings.HasSuffix(senderStr, "@lid") {
		// 1. Prefer SenderAlt — no DB round-trip, filled by whatsmeow.
		altStr := senderAlt.String()
		if strings.HasSuffix(altStr, pnSuffix) {
			return StripDeviceIDFromJID(altStr), "pn"
		}

		// 2. Persisted LID→PN mapping.
		if client != nil && client.Store != nil && client.Store.LIDs != nil {
			resolvedJID, err := client.Store.LIDs.GetPNForLID(context.Background(), senderJID)
			if err == nil && !resolvedJID.IsEmpty() && strings.HasSuffix(resolvedJID.String(), pnSuffix) {
				return StripDeviceIDFromJID(resolvedJID.String()), "pn"
			}
		}

		// 3. Chat JID (only valid for 1:1 chats where chat == peer PN).
		chatStr := chatJID.String()
		if strings.HasSuffix(chatStr, pnSuffix) {
			return StripDeviceIDFromJID(chatStr), "pn"
		}

		// 4. Unresolvable (typically a group LID-only participant). Surface the
		// raw @lid honestly and flag it so consumers don't treat `from` as a
		// phone number.
		return StripDeviceIDFromJID(senderStr), "lid"
	}

	// Default: stripped sender.
	return StripDeviceIDFromJID(senderStr), "pn"
}
