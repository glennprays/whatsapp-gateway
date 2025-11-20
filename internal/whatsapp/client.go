package whatsapp

import (
	"context"
	"errors"
	"fmt"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
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
	client := Clients[phoneNumber]
	if client == nil {
		return errors.New(constant.ErrClientNotFound)
	}
	client.Disconnect()
	err := client.Connect()
	if err != nil {
		return err
	}
	return nil
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

	return "", 0, errors.New(constant.ErrClientNotFound)
}

func LoginStatus(phoneNumber string) (bool, error) {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]
		return client.IsLoggedIn(), nil
	}
	return false, errDomain.NewError(errDomain.ErrNotFound, errors.New("client not found"))
}

func Logout(ctx context.Context, phoneNumber string) error {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]
		err := client.Logout(ctx)
		if err != nil {
			log.Errorf("Failed to logout client %s: %v", MaskedPhoneNumber(phoneNumber), err)
			client.Disconnect()
			if err := client.Store.Delete(ctx); err != nil {
				log.Errorf("Failed to delete client store %s: %v", MaskedPhoneNumber(phoneNumber), err)
			}
			return nil
		}
		return nil
	}
	return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
}
