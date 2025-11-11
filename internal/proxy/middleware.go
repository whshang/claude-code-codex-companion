package proxy

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)


func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := generateRequestID()
		c.Set("request_id", requestID)
		c.Set("start_time", start)

		c.Next()
	}
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}