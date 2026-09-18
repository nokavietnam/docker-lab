package middleware

import (
	"log"
	"time"
	"uuid"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		c.Writer.Header().Add("X-Request-ID", reqID)
		c.Set("request_id", reqID)

		c.Next()

		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path

		log.Printf("[HTTP] | %3d | %13v | %15s | %-7s %s | ReqID: %s",
			statusCode,
			latency,
			clientIP,
			method,
			path,
			reqID,
		)
	}
}
