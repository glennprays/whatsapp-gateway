package whatsapp_usecase

import (
	"context"
	"errors"

	customLog "github.com/glennprays/log"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
)

// WhatsappAuthUsecase handles WhatsApp authentication business logic
type WhatsappAuthUsecase struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
}

// NewWhatsappAuthUsecase creates a new WhatsApp authentication usecase
func NewWhatsappAuthUsecase(manager whatsapp.Manager, logger *customLog.Logger) *WhatsappAuthUsecase {
	return &WhatsappAuthUsecase{
		whatsappManager: manager,
		logger:          logger,
	}
}

// QRCodeResponse contains QR code data and timeout
type QRCodeResponse struct {
	QRCode  string
	Timeout int
}

// PairCodeResponse contains pair code and expiration
type PairCodeResponse struct {
	PairCode  string
	ExpiresIn int
}

// LoginQRCode generates QR code for WhatsApp login
func (uc *WhatsappAuthUsecase) LoginQRCode(ctx context.Context, traceID, phoneNumber, format string) (*QRCodeResponse, error) {
	// Validate format
	formatMap := map[string]bool{
		"json": true,
		"html": true,
	}

	if format == "" {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("format params is required"))
	}

	if _, ok := formatMap[format]; !ok {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("invalid format params"))
	}

	// Register client and get QR code
	uc.whatsappManager.RegisterClient(ctx, traceID, phoneNumber)

	qrCode, timeout, err := uc.whatsappManager.LoginQRCode(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to generate QR code", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return &QRCodeResponse{
		QRCode:  qrCode,
		Timeout: timeout,
	}, nil
}

// LoginPairCode generates pairing code for WhatsApp login
func (uc *WhatsappAuthUsecase) LoginPairCode(ctx context.Context, traceID, phoneNumber string) (*PairCodeResponse, error) {
	uc.whatsappManager.RegisterClient(ctx, traceID, phoneNumber)

	pairCode, timeout, err := uc.whatsappManager.LoginPairCode(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to generate pair code", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return &PairCodeResponse{
		PairCode:  pairCode,
		ExpiresIn: timeout,
	}, nil
}

// GetLoginStatus returns the current login status
func (uc *WhatsappAuthUsecase) GetLoginStatus(ctx context.Context, traceID, phoneNumber string) (bool, error) {
	status, err := uc.whatsappManager.LoginStatus(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to get login status", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return false, err
	}

	return status, nil
}

// Logout handles WhatsApp logout
func (uc *WhatsappAuthUsecase) Logout(ctx context.Context, traceID, phoneNumber string) error {
	err := uc.whatsappManager.Logout(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to logout", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return err
	}

	return nil
}

// Reconnect handles WhatsApp reconnection
func (uc *WhatsappAuthUsecase) Reconnect(ctx context.Context, traceID, phoneNumber string) error {
	err := uc.whatsappManager.Reconnect(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to reconnect", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(phoneNumber)),
		}, customLog.Error(err))
		return err
	}

	return nil
}
