package actions

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/manishiitg/actions/util"
)

func (e *etcdCoordinator) InitApi(port string) error {
	r := gin.Default()
	r.Use(cors.Default())
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/status", func(c *gin.Context) {
		c.JSON(200, *util.GetActionStatus())
	})

	r.GET("/stop", func(c *gin.Context) {
		status := util.GetActionStatus()
		if status.IsActive {
			if status.ActionType == "tracktodisk" {
				close(e.diskActionCancel)
			}
			if status.ActionType == "loadtest" {
				close(e.loadActionCancel)
			}
			if status.ActionType == "rtmptotrack" {
				close(e.rtmpActionCancel)
			}
			if status.ActionType == "tracktortp" {
				close(e.streamActionCancel)
			}
			if status.ActionType == "mirrorsfu" {
				close(e.mirrorActionCancel)
			}
			e.engine = nil
			util.CloseAction()
		}

	})

	mirrorr := r.Group("mirror")
	{
		mirrorr.GET("/stop", func(c *gin.Context) {
			stopMirror(c, e)
		})
		mirrorr.GET("/sync/:session1/:session2", func(c *gin.Context) {
			if e.engine != nil {
				c.String(http.StatusOK, "Engine Already Used!")
				return
			}
			startMirror(c, e)
		})
	}
	loadtestr := r.Group("loadtest")
	{
		loadtestr.GET("/stop", func(c *gin.Context) {
			stopLoadTest(c, e)
		})
		loadtestr.GET("/:session", func(c *gin.Context) {
			if e.engine != nil {
				c.String(http.StatusOK, "Engine Already Used!")
				return
			}
			startLoadTest(c, e)
		})
		loadtestr.GET("/stats", func(c *gin.Context) {
			stats(c, e)
		})
	}

	streamr := r.Group("stream")
	{

		streamr.GET("/live/:session/:rtmp", func(c *gin.Context) {
			// if e.engine != nil {
			// 	c.String(http.StatusOK, "Engine Already Used!")
			// 	return
			// }
			startRealStream(c, e)
		})
		streamr.GET("/demo/:session", func(c *gin.Context) {
			// if e.engine != nil {
			// 	c.String(http.StatusOK, "Engine Already Used!")
			// 	return
			// }
			startTestStream(c, e)
		})
		streamr.GET("/stop", func(c *gin.Context) {
			stopStream(c, e)
		})
	}

	diskr := r.Group("disk")
	{
		diskr.GET("/:session", func(c *gin.Context) {
			// if e.engine != nil {
			// 	c.String(http.StatusOK, "Engine Already Used!")
			// 	return
			// }
			saveToDisk(c, e)
		})
		diskr.GET("/stop", func(c *gin.Context) {
			stopDisk(c, e)
		})
	}

	rtpr := r.Group("rtmp")
	{
		rtpr.GET("/live/:session/:rtmp", func(c *gin.Context) {
			if e.engine != nil {
				c.String(http.StatusOK, "Engine Already Used!")
				return
			}
			startActualRtmp(c, e)
		})
		rtpr.GET("/demo/:session", func(c *gin.Context) {
			if e.engine != nil {
				c.String(http.StatusOK, "Engine Already Used!")
				return
			}
			startDemoRtmp(c, e)
		})
		rtpr.GET("/stop", func(c *gin.Context) {
			stopRtmp(c, e)
		})
	}

	return r.Run(port)
}
