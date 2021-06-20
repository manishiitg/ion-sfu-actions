package actions

import (
	"net/http"

	"github.com/gin-gonic/gin"
	mirrorsfu "github.com/manishiitg/actions/mirror-sfu"
)

func stopMirror(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.engine != nil {

		e.engine = nil
	}
	close(e.mirrorActionCancel)
	c.Status(http.StatusOK)
}
func startMirror(c *gin.Context, e *etcdCoordinator) {
	if e.engine != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": "already_running",
		})
	} else {
		e.mu.Lock()
		defer e.mu.Unlock()
		cancel := make(chan struct{})
		go mirrorsfu.Init(c.Param("session1"), c.Param("session2"), e.serverIp, cancel)
		e.mirrorActionCancel = cancel
		c.Status(http.StatusOK)
	}
}
