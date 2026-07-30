package whatsapp_handler

import (
	"fmt"
	"net/http"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
)

type WhatsappAuthHandler struct {
	whatsappAuthUsecase *whatsapp_usecase.WhatsappAuthUsecase
	logger              *customLog.Logger
}

func NewWhatsappAuthHandler(whatsappAuthUsecase *whatsapp_usecase.WhatsappAuthUsecase, logger *customLog.Logger) *WhatsappAuthHandler {
	return &WhatsappAuthHandler{
		whatsappAuthUsecase: whatsappAuthUsecase,
		logger:              logger,
	}
}

func (h *WhatsappAuthHandler) LoginQRCode(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	format := c.Params("format")

	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	ctx := c.Context()

	// Call usecase
	response, err := h.whatsappAuthUsecase.LoginQRCode(ctx, traceID, phoneNumber, format)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// Format response based on format parameter
	switch format {
	case "json":
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"qr_code": response.QRCode,
			"timeout": response.Timeout,
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
`, response.QRCode, response.Timeout)
		return c.Status(http.StatusOK).Type("html").SendString(html)
	}
	return nil
}

func (h *WhatsappAuthHandler) LoginPairCode(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	ctx := c.Context()

	// Call usecase
	response, err := h.whatsappAuthUsecase.LoginPairCode(ctx, traceID, phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"pair_code":  response.PairCode,
		"expires_in": response.ExpiresIn,
	})
}

func (h *WhatsappAuthHandler) GetLoginStatus(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	// Call usecase
	status, err := h.whatsappAuthUsecase.GetLoginStatus(c.Context(), traceID, phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	// "authenticated" is a deprecated alias kept for back-compat; it reports the
	// WhatsApp device-pairing state (IsLoggedIn), not API-token validity. Prefer
	// "logged_in". "banned"/"ban_expires_at" surface an active temporary ban.
	resp := fiber.Map{
		"authenticated": status.LoggedIn,
		"logged_in":     status.LoggedIn,
		"banned":        status.Banned,
	}
	if status.BanExpiresAt != nil {
		resp["ban_expires_at"] = status.BanExpiresAt.UTC().Format(time.RFC3339)
	}
	return c.Status(http.StatusOK).JSON(resp)
}

func (h *WhatsappAuthHandler) Logout(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	// Call usecase
	err := h.whatsappAuthUsecase.Logout(c.Context(), traceID, phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
	})
}

func (h *WhatsappAuthHandler) Reconnect(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		h.logger.Error(traceID, constant.ErrPhoneNumberNotFound, nil)
		return nil
	}

	// Call usecase
	err := h.whatsappAuthUsecase.Reconnect(c.Context(), traceID, phoneNumber)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
	})
}
