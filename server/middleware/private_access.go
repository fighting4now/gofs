package middleware

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/no-src/gofs/contract"
	"github.com/no-src/gofs/server"
	"github.com/no-src/log"
)

type privateAccessHandler struct {
	logger log.Logger
}

// NewPrivateAccessHandlerFunc returns a middleware that only allows to access http resource from loop back ip or private ip
func NewPrivateAccessHandlerFunc(logger log.Logger) gin.HandlerFunc {
	return (&privateAccessHandler{
		logger: logger,
	}).Handle
}

func (h *privateAccessHandler) Handle(c *gin.Context) {
	ip := net.ParseIP(c.ClientIP())
	if !ip.IsPrivate() && !ip.IsLoopback() {
		h.logger.Warn("access deny, client ip is [%s], path is [%s]", c.ClientIP(), c.FullPath())
		c.Abort()
		c.JSON(http.StatusUnauthorized, server.NewApiResult(contract.AccessDeny, contract.AccessDenyDesc, nil))
	}
}
