package middlewares

import "github.com/gin-gonic/gin"

// CORS handle cors.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		// If credentials are needed (optional)
		c.Header("Access-Control-Allow-Credentials", "true")

		c.Next()
	}
}
