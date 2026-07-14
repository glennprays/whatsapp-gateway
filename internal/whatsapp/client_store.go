package whatsapp

import (
	"sync"

	"go.mau.fi/whatsmeow"
)

type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]*whatsmeow.Client
	// jids caches each account's last-known device JID. whatsmeow nils out
	// Store.ID (and cascade-deletes the device row) concurrently during a
	// logout, so a lifecycle event handler cannot safely read the live client
	// for the JID it needs to address a webhook — it reads the cache instead.
	jids map[string]string
}

func NewClientStore() *ClientStore {
	return &ClientStore{
		clients: make(map[string]*whatsmeow.Client),
		jids:    make(map[string]string),
	}
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
