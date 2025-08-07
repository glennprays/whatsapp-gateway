package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"runtime"
	"strings"

	qrCode "github.com/skip2/go-qrcode"
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
	qrChanCode := make(chan string)
	qrChanTimeout := make(chan int)

	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				select {
				case qrChanCode <- evt.Code:
				case <-ctx.Done():
					return
				}

				select {
				case qrChanTimeout <- int(evt.Timeout.Seconds()):
				case <-ctx.Done():
					return
				}
				return
			}
		}
	}()

	var (
		code    string
		timeout int
	)

	select {
	case code = <-qrChanCode:
	case <-ctx.Done():
		return "", 0, errors.New("context cancelled while waiting for QR code")
	}

	select {
	case timeout = <-qrChanTimeout:
	case <-ctx.Done():
		return "", 0, errors.New("context cancelled while waiting for QR timeout")
	}

	qrPng, err := qrCode.Encode(code, qrCode.Medium, 256)
	if err != nil {
		return "", 0, err
	}

	return base64.StdEncoding.EncodeToString(qrPng), timeout, nil
}
