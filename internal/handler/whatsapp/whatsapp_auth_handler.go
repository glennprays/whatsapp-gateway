package whatsapp_handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
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
}

func NewWhatsappAuthHandler(manager whatsapp.Manager) *WhatsappAuthHandler {
	return &WhatsappAuthHandler{
		whatsappManager: manager,
	}
}

func (h *WhatsappAuthHandler) LoginQRCode(c *gin.Context) {
	format := c.Param("format")
	if format == "" {
		err := errDomain.NewError(errDomain.ErrBadRequest, errors.New("format params is required"))
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	if _, ok := formatMap[format]; !ok {
		err := errDomain.NewError(errDomain.ErrBadRequest, errors.New("invalid format params"))
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
	}

	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error("Phone number not found in context")
		c.Abort()
		return
	}

	ctx := c.Request.Context()
	h.whatsappManager.RegisterClient(ctx, phoneNumber)

	qrCode, timeout, err := h.whatsappManager.LoginQRCode(ctx, phoneNumber)
	if err != nil {
		log.Errorf("Failed to generate QR code for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err))
		c.JSON(httpErr.Status, httpErr)
		return
	}

	switch format {
	case "json":
		c.JSON(http.StatusOK, gin.H{
			"qr_code": qrCode,
			"timeout": timeout,
		})
		return
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
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		return
	}
}

func (h *WhatsappAuthHandler) GetLoginStatus(c *gin.Context) {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error("Phone number not found in context")
		c.Abort()
		return
	}

	status, err := h.whatsappManager.LoginStatus(c.Request.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to get login status for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": status,
	})
}

func (h *WhatsappAuthHandler) Logout(c *gin.Context) {
	phoneNumber, ok := utils.MustGetPhoneNumber(c)
	if !ok {
		log.Error("Phone number not found in context")
		c.Abort()
		return
	}

	err := h.whatsappManager.Logout(c.Request.Context(), phoneNumber)
	if err != nil {
		log.Errorf("Failed to logout for phone number %s: %v", phoneNumber, err)
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
