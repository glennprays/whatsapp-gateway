package whatsapp

import (
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
)

// currentQR is the pairing QR currently being served for one phone number.
// WhatsApp QRs are bound to the connection that minted them: any Disconnect
// invalidates the ref, and a re-mint kills every previously issued code. The
// cache lets repeated requests within the validity window return the SAME
// live QR instead of silently superseding the one on screen.
type currentQR struct {
	png       string    // full data:image/png;base64,… value
	timeout   int       // seconds whatsmeow reported for this code
	expiresAt time.Time // mint time + timeout - 2s safety margin
}

type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]*whatsmeow.Client
	// jids caches each account's last-known device JID. whatsmeow nils out
	// Store.ID (and cascade-deletes the device row) concurrently during a
	// logout, so a lifecycle event handler cannot safely read the live client
	// for the JID it needs to address a webhook — it reads the cache instead.
	jids map[string]string
	// currentQRs holds the live pairing QR per phone number (in-memory only,
	// like clients; a gateway restart drops it and the next request re-mints).
	currentQRs map[string]*currentQR
}

func NewClientStore() *ClientStore {
	return &ClientStore{
		clients:    make(map[string]*whatsmeow.Client),
		jids:       make(map[string]string),
		currentQRs: make(map[string]*currentQR),
	}
}

// GetCurrentQR returns the still-valid cached QR for a phone number, or nil
// when absent or expired. Expiry is checked here so callers never re-check.
func (cs *ClientStore) GetCurrentQR(phoneNumber string) *currentQR {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	qr, ok := cs.currentQRs[phoneNumber]
	if !ok || time.Now().After(qr.expiresAt) {
		return nil
	}
	return qr
}

func (cs *ClientStore) SetCurrentQR(phoneNumber string, qr currentQR) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.currentQRs[phoneNumber] = &qr
}

func (cs *ClientStore) DeleteCurrentQR(phoneNumber string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.currentQRs, phoneNumber)
}

// SetJID records an account's device JID while the client is alive, so a later
// teardown event can still resolve it without racing whatsmeow's Store.Delete.
func (cs *ClientStore) SetJID(phoneNumber, jid string) {
	if jid == "" {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.jids[phoneNumber] = jid
}

// JID returns the last-known device JID for an account, or "" if never seen.
func (cs *ClientStore) JID(phoneNumber string) string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.jids[phoneNumber]
}

func (cs *ClientStore) Get(phoneNumber string) *whatsmeow.Client {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.clients[phoneNumber]
}

func (cs *ClientStore) Set(phoneNumber string, client *whatsmeow.Client) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.clients[phoneNumber] = client
}

func (cs *ClientStore) Delete(phoneNumber string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.clients, phoneNumber)
}

func (cs *ClientStore) GetAll() map[string]*whatsmeow.Client {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	cp := make(map[string]*whatsmeow.Client, len(cs.clients))
	for k, v := range cs.clients {
		cp[k] = v
	}
	return cp
}
