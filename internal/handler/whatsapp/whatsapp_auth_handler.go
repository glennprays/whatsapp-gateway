package whatsapp_handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	customLog "github.com/glennprays/log"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	log "github.com/sirupsen/logrus"
)

var formatMap map[string]bool

func init() {
	formatMap = map[string]bool{
		"json": true,
		"html": true,
	}
}

type WhatsappAuthHandler struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
}

func NewWhatsappAuthHandler(manager whatsapp.Manager, logger *customLog.Logger) *WhatsappAuthHandler {
	return &WhatsappAuthHandler{
		whatsappManager: manager,
		logger:          logger,
	}
}

func (h *WhatsappAuthHandler) LoginQRCode(c *fiber.Ctx) error {
	format := c.Params("format")
	if format == "" {
		err := errDomain.NewError(errDomain.ErrBadRequest, errors.New("format params is required"))
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	if _, ok := formatMap[format]; !ok {
		err := errDomain.NewError(errDomain.ErrBadRequest, errors.New("invalid format params"))
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	ctx := c.Context()
	h.whatsappManager.RegisterClient(ctx, phoneNumber)

	qrCode, timeout, err := h.whatsappManager.LoginQRCode(ctx, phoneNumber)
	if err != nil {
		log.Errorf("Failed to generate QR code for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	switch format {
	case "json":
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"qr_code": qrCode,
			"timeout": timeout,
		})
	case "html":
		html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>WhatsApp QR Login</title>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body {
			font-family: Arial, sans-serif;
			background-color: #f7f7f7;
			color: #333;
			display: flex;
			flex-direction: column;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
		}
		.container {
			background-color: #fff;
			padding: 24px 32px;
			border-radius: 12px;
			box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
			text-align: center;
		}
		img {
			width: 280px;
			height: 280px;
			margin-bottom: 16px;
			border: 1px solid #ccc;
			border-radius: 8px;
		}
		p {
			margin: 6px 0;
			font-size: 16px;
		}
	</style>
</head>
<body>
	<div class="container">
		<img src="%s" alt="QR Code" />
		<p>Scan this QR code with your WhatsApp app to log in.</p>
		<p>This QR code will expire in %d seconds.</p>
	</div>
</body>
</html>
`, qrCode, timeout)
		return c.Status(http.StatusOK).Type("text/html", "utf-8").Send([]byte(html))
	}
	return nil
}

func (h *WhatsappAuthHandler) LoginPairCode(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	ctx := c.Context()

	h.whatsappManager.RegisterClient(ctx, phoneNumber)
	pairCode, timeout, err := h.whatsappManager.LoginPairCode(ctx, phoneNumber)
	if err != nil {
		log.Errorf("Failed to generate pair code for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err))
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"pair_code":  pairCode,
		"expires_in": timeout,
	})
}

func (h *WhatsappAuthHandler) GetLoginStatus(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	status, err := h.whatsappManager.LoginStatus(c.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to get login status for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"authenticated": status,
	})
}

func (h *WhatsappAuthHandler) Logout(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	err := h.whatsappManager.Logout(c.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to logout for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
	})
}

func (h *WhatsappAuthHandler) Reconnect(c *fiber.Ctx) error {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error(constant.ErrPhoneNumberNotFound)
		return nil
	}

	err := h.whatsappManager.Reconnect(c.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to reconnect for phone number %s: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
	})
}
