package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"trojan-panel-core/bootstrap"
)

func BootstrapHealth(c *gin.Context) {
	if !bootstrap.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "waiting_for_web_mtls"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
