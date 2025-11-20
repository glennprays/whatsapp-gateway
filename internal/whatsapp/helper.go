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
