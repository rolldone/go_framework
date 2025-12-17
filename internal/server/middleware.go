package server

import (
	"fmt"

	authjwt "go_framework/internal/auth"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware parses optional Authorization: Bearer <token> header and
// injects `user_id` into the context when a valid access token is provided.
// It does not enforce authentication — handlers should enforce as needed.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth != "" {
			var token string
			if n, _ := fmt.Sscanf(auth, "Bearer %s", &token); n == 1 {
				if sub, err := authjwt.ParseAccessToken(token); err == nil {
					c.Set("user_id", sub)
				}
			}
		}
		c.Next()
	}
}
