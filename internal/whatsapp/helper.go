package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"runtime"
	"strings"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
)

func WhatsappDecomposeJID(id string) string {
	if strings.ContainsRune(id, '@') {
		buffers := strings.Split(id, "@")
		id = buffers[0]
	}

	if id[0] == '+' {
		id = id[1:]
	}

	return id
}

func MaskedJID(jid string) string {
	if len(jid) < 4 {
		return jid
	}
	return jid[:len(jid)-4] + "xxxx"
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
