package whatsapp

import (
	"sync"

	"go.mau.fi/whatsmeow"
)

type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]*whatsmeow.Client
}

func NewClientStore() *ClientStore {
	return &ClientStore{
		clients: make(map[string]*whatsmeow.Client),
	}
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
