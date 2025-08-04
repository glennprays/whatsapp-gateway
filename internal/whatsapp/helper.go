package whatsapp

import (
	"runtime"
	"strings"
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
