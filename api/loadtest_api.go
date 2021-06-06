package actions

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	loadtest "github.com/manishiitg/actions/loadtest"
)

func stopLoadTest(c *gin.Context, e *etcdCoordinator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.engine != nil {
		close(e.loadActionCancel)
		e.engine = nil
	}
	c.Status(http.StatusOK)
}

func startLoadTest(c *gin.Context, e *etcdCoordinator) {
	clients := c.Query("clients")
	no := 1
	if clients != "" {
		x, err := strconv.Atoi(clients)
		if err == nil {
			no = x
		}
	}

	role := c.Query("role")
	if len(role) == 0 || role == "pubsub" {
		role = "pubsub"
	} else {
		role = "sub"
	}

	qcycle := c.Query("cycle")
	cycle := 0
	if len(qcycle) != 0 {
		x, err := strconv.Atoi(qcycle)
		if err == nil {
			cycle = x
		}
	}

	qrooms := c.Query("rooms")
	rooms := -1
	if len(qrooms) != 0 {
		x, err := strconv.Atoi(qrooms)
		if err == nil {
			rooms = x
		}
	}

	file := c.Query("file")
	if len(file) == 0 {
		file = "default"
	} else {
		file = c.Query("file")
	}

	qcap := c.Query("capacity")
	capacity := -1
	if len(qcap) != 0 {
		x, err := strconv.Atoi(qcap)
		if err == nil {
			capacity = x
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	cancel := make(chan struct{})
	engine := loadtest.InitLoadTestApi(e.serverIp, c.Param("session"), no, role, cycle, rooms, file, capacity, cancel)
	e.engine = engine
	e.loadActionCancel = cancel
	c.Status(http.StatusOK)
}

func stats(c *gin.Context, e *etcdCoordinator) {
	if e.engine != nil {
		clients, totalRecvBW, totalSendBW := e.engine.GetStat()
		e.getHostLoad()
		c.JSON(http.StatusOK, gin.H{
			"clients":     clients,
			"totalRecvBW": totalRecvBW,
			"totalSendBW": totalSendBW,
			"engine":      1,
			"hostload":    e.getHostLoad(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"clients":     0,
			"totalRecvBW": 0,
			"totalSendBW": 0,
			"engine":      0,
			"hostload":    e.getHostLoad(),
		})
	}
}
