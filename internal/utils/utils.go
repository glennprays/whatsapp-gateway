package utils

import "github.com/gofiber/fiber/v2"

func GetIPFromFiberCtx(c *fiber.Ctx) string {
	if ip := c.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := c.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return c.IP()
}
