package actions

import (
	"net/http"

	"github.com/gin-gonic/gin"
	tracktodisk "github.com/manishiitg/actions/tracktodisk"
)

func saveToDisk(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel := make(chan struct{})
	session := c.Param("session")
	engine := tracktodisk.InitApi(e.serverIp, session, "webm", cancel)
	e.diskActionCancel = cancel
	e.engine = engine
	c.Status(http.StatusOK)
}

func stopDisk(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.engine != nil {
		close(e.diskActionCancel)
		e.engine = nil
	}
	c.Status(http.StatusOK)
}
