package whatsapp

import (
	"context"
	"errors"
	"fmt"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type (
	client struct {
		container  *sqlstore.Container
		cfg        *config.Config
		repository WhatsAppRepository
	}
	Client interface {
		InitClient(phoneNumber string, device *store.Device, eventHandler func(string, any))
		Reconnect(phoneNumber string) error
		LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error)
		LoginPairCode(ctx context.Context, phoneNumber string) (string, int, error)
		LoginStatus(phoneNumber string) (bool, error)
		Logout(ctx context.Context, phoneNumber string) error
		GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error)
		SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error
		DeleteWebhookURL(ctx context.Context, phoneNumber string) error
	}
)

func NewClient(container *sqlstore.Container, cfg *config.Config, repo WhatsAppRepository) Client {
	return &client{
		container:  container,
		cfg:        cfg,
		repository: repo,
	}
}

func (c *client) InitClient(phoneNumber string, device *store.Device, eventHandler func(string, any)) {
	binary.IndentXML = true
	if Clients[phoneNumber] == nil {
		if device == nil {
			log.Infof("Creating new device for Phone Number: %s", MaskedPhoneNumber(phoneNumber))
			device = c.container.NewDevice()
		}
		store.DeviceProps.Os = proto.String(c.cfg.WhatsappDeviceLabel)
		store.DeviceProps.RequireFullSync = proto.Bool(false)

		client := whatsmeow.NewClient(device, waLog.Stdout("Client-login", c.cfg.WhatsmeowLogLevel, true))
		client.AddEventHandler(func(evt any) {
			eventHandler(phoneNumber, evt)
		})
		client.EnableAutoReconnect = true
		client.AutoTrustIdentity = true

		Clients[phoneNumber] = client
	}
}

func (c *client) Reconnect(phoneNumber string) error {
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

func (c *client) LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error) {
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

		err := c.Reconnect(phoneNumber)
		if err != nil {
			return "", 0, err
		}

		return "", 0, errDomain.NewError(errDomain.ErrConflict, errors.New(constant.ErrClientAlreadyLoggedIn))
	}

	return "", 0, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
}

func (c *client) LoginPairCode(ctx context.Context, phoneNumber string) (string, int, error) {
	client := Clients[phoneNumber]
	if client == nil {
		return "", 0, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	client.Disconnect()
	if client.Store.ID == nil {
		err := client.Connect()
		if err != nil {
			return "", 0, err
		}

		pairCode, err := client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, fmt.Sprintf("Chrome (%s)", WhatsAppGetUserOS()))
		if err != nil {
			return "", 0, err
		}
		return pairCode, 160, nil
	}

	err := c.Reconnect(phoneNumber)
	if err != nil {
		return "", 0, err
	}

	return "", 0, errDomain.NewError(errDomain.ErrConflict, errors.New(constant.ErrClientAlreadyLoggedIn))
}

func (c *client) LoginStatus(phoneNumber string) (bool, error) {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]
		return client.IsLoggedIn(), nil
	}
	return false, errDomain.NewError(errDomain.ErrNotFound, errors.New("client not found"))
}

func (c *client) Logout(ctx context.Context, phoneNumber string) error {
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

func (c *client) GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error) {
	client := Clients[phoneNumber]
	if client == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	webhookURL, err := c.repository.GetWebhook(ctx, client.Store.ID.String())
	if err != nil {
		log.Errorf("Failed to get webhook URL for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if webhookURL == nil {
		return nil, nil
	}

	return &webhookURL.Url, nil
}

func (c *client) SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	err := c.repository.SetWebhook(ctx, client.Store.ID.String(), webhook.Url, webhook.HmacSecret)
	if err != nil {
		log.Errorf("Failed to set webhook URL for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return nil
}

func (c *client) DeleteWebhookURL(ctx context.Context, phoneNumber string) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	err := c.repository.DeleteWebhook(ctx, client.Store.ID.String())
	if err != nil {
		log.Errorf("Failed to delete webhook URL for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return nil
}
