package whatsapp

import (
	"sync"
	"time"

	customLog "github.com/glennprays/log"
	"go.mau.fi/whatsmeow"
)

// DisconnectAll cleanly disconnects every live whatsmeow client (one per
// Account) during shutdown, concurrently and bounded by timeout. whatsmeow
// flushes session state on Disconnect, so this MUST run before the shared
// database is closed. A client whose Disconnect hangs is left behind rather
// than blocking shutdown past the grace period.
func DisconnectAll(logger *customLog.Logger, traceID string, timeout time.Duration) {
	all := clients.GetAll()
	if len(all) == 0 {
		return
	}
	logger.Info(traceID, "Disconnecting WhatsApp clients...", nil, customLog.Int("count", len(all)))

	var wg sync.WaitGroup
	for phone, client := range all {
		if client == nil {
			continue
		}
		wg.Add(1)
		go func(phone string, c *whatsmeow.Client) {
			defer wg.Done()
			c.Disconnect()
			logger.Info(traceID, "Disconnected WhatsApp client", nil,
				customLog.String("phone_number", MaskedPhoneNumber(phone)))
		}(phone, client)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info(traceID, "All WhatsApp clients disconnected", nil)
	case <-time.After(timeout):
		logger.Warn(traceID, "Timed out disconnecting WhatsApp clients; proceeding with shutdown", nil)
	}
}
