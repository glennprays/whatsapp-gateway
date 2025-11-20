package whatsapp

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

func InitClient(phoneNumber string, device *store.Device) {
	binary.IndentXML = true
	if Clients[phoneNumber] == nil {
		if device == nil {
			log.Infof("Creating new device for Phone Number: %s", MaskedPhoneNumber(phoneNumber))
			device = container.NewDevice()
		}
		store.DeviceProps.Os = proto.String(cfg.WhatsappDeviceLabel)
		store.DeviceProps.RequireFullSync = proto.Bool(false)

		client := whatsmeow.NewClient(device, waLog.Stdout("Client-login", cfg.WhatsmeowLogLevel, true))
		client.AddEventHandler(func(evt any) {
			HandleEvent(phoneNumber, evt)
		})
		client.EnableAutoReconnect = true
		client.AutoTrustIdentity = true

		Clients[phoneNumber] = client
	}
}

func Reconnect(phoneNumber string) error {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]
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

func LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error) {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]

		client.Disconnect()
		if client.Store.ID == nil {
			qrChanGenerate, _ := client.GetQRChannel(context.Background())
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

		err := Reconnect(phoneNumber)
		if err != nil {
			return "", 0, err
		}

		return "", 0, errors.New("client already logged in")
	}

	return "", 0, errors.New("client not found, please register first")
}
