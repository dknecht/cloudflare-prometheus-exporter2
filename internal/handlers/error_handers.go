package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lablabs/cloudflare-exporter/internal/logging"
)

// ErrorHandler middleware handles errors and logs them
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			logging.Error("Request error", map[string]interface{}{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
				"error":  err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	}
}
