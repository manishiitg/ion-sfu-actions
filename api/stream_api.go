package actions

import (
	b64 "encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	tracktortp "github.com/manishiitg/actions/tracktortp"
	log "github.com/pion/ion-log"
)

func startRealStream(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel := make(chan struct{})
	session := c.Param("session")
	rtmp, _ := b64.StdEncoding.DecodeString(c.Param("rtmp"))
	engine, err := tracktortp.InitApi(e.serverIp, session, string(rtmp), cancel)
	if err != nil {
		log.Infof("error in init api %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		close(e.streamActionCancel)
		return
	}
	e.streamActionCancel = cancel
	e.engine = engine
	c.Status(http.StatusOK)
}

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
