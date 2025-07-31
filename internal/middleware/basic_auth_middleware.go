package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func BasicAuthMiddleware(expectedUser, expectedPassword string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, password, hasAuth := c.Request.BasicAuth()

		if hasAuth && user == expectedUser && password == expectedPassword {
			c.Next()
		} else {
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}
