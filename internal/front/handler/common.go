package handler

import "github.com/gin-gonic/gin"

func channelIDFromContext(c *gin.Context) *string {
	if val, ok := c.Get("sales_channel_id"); ok {
		if ptr, ok := val.(*string); ok {
			return ptr
		}
	}
	return nil
}
