package actions

import (
	"net/http"

	"github.com/gin-gonic/gin"
	tracktortp "github.com/manishiitg/actions/tracktortp"
)

func startTestStream(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel := make(chan struct{})
	session := c.Param("session")
	engine, err := tracktortp.InitApi(e.serverIp, session, "", cancel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		close(e.streamActionCancel)
		return
	}
	e.streamActionCancel = cancel
	e.engine = engine
	c.Status(http.StatusOK)
}

func stopStream(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.engine != nil {
		close(e.streamActionCancel)
		e.engine = nil
	}
	c.Status(http.StatusOK)
}
