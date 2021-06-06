package actions

import (
	b64 "encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/manishiitg/actions/rtmptotrack"
)

func startActualRtmp(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel := make(chan struct{})
	session := c.Param("session")
	rtmp, _ := b64.StdEncoding.DecodeString(c.Param("rtmp"))
	engine := rtmptotrack.Init(session, e.serverIp, string(rtmp), cancel)
	e.rtmpActionCancel = cancel
	e.engine = engine
	c.Status(http.StatusOK)
}

func startDemoRtmp(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel := make(chan struct{})
	session := c.Param("session")
	engine := rtmptotrack.Init(session, e.serverIp, "demo", cancel)
	e.rtmpActionCancel = cancel
	e.engine = engine
	c.Status(http.StatusOK)
}

func stopRtmp(c *gin.Context, e *etcdCoordinator) {

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.engine != nil {
		close(e.rtmpActionCancel)
		e.engine = nil
	}
	c.Status(http.StatusOK)

}
