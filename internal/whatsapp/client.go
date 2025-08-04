package whatsapp

import (
	"errors"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

// Init Whatsmeow client, wrap login
func InitClient(jid string, device *store.Device) {
	if Clients[jid] == nil {
		if device == nil {
			device = container.NewDevice()
		}
		store.DeviceProps.Os = proto.String(WhatsAppGetUserOS())
		store.DeviceProps.RequireFullSync = proto.Bool(false)

		client := whatsmeow.NewClient(device, nil)
		client.AddEventHandler(func(evt any) {
			HandleEvent(jid, evt)
		})
		client.EnableAutoReconnect = true
		client.AutoTrustIdentity = true

		Clients[jid] = client
	}
}

func Reconnect(jid string) error {
	if Clients[jid] != nil {
		client := Clients[jid]
		client.Disconnect()

		if client != nil {
			err := client.Connect()
			if err != nil {
				return err
			}
			return nil
		}
		return errors.New("client Store is empty, please re-login")
	}

	return errors.New("client not found, place re-login")
}
