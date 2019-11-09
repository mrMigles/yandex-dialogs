package main

import (
	"github.com/gin-gonic/gin"
)

type VoiceMail struct {
}

func (handler VoiceMail) handleRequest() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.String(200, "Ok")
	}
}
