package whatsapp

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

func InitClient(jid string, device *store.Device) {
	binary.IndentXML = true
	if Clients[jid] == nil {
		if device == nil {
			log.Info("Device for JID %s is nil, creating a new device", jid)
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

func LoginQRCode(ctx context.Context, jid string) (string, int, error) {
	if Clients[jid] != nil {
		client := Clients[jid]

		client.Disconnect()
		if client.Store.ID == nil {
			qrChanGenerate, _ := client.GetQRChannel(ctx)
			err := client.Connect()
			if err != nil {
				return "", 0, err
			}

			qrImage, qrTimeout, err := WhatsappGenerateQRCode(ctx, qrChanGenerate)
			if err != nil {
				return "", 0, err
			}

			return fmt.Sprintf(`data:image/png;base64,%s`, qrImage), qrTimeout, nil
		}

		err := Reconnect(jid)
		if err != nil {
			return "", 0, err
		}

		return "", 0, errors.New("client already logged in")
	}

	return "", 0, errors.New("client not found, please register first")
}
