package handlers

import "github.com/gin-gonic/gin"

// HealthCheck function handles health check.
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "healthy",
	})
}
