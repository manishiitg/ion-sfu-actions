package actions

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

	mirrorr := r.Group("mirror")
	{
		mirrorr.GET("/stop", func(c *gin.Context) {
			stopMirror(c, e)
		})
		mirrorr.GET("/sync/:session1/:session2", func(c *gin.Context) {
			startMirror(c, e)
		})
		mirrorr.GET("/syncsfu/:session1/:session2/:addr1/:addr2", func(c *gin.Context) {
			startMirrorWithAddr(c, e)
		})

	}
	loadtestr := r.Group("loadtest")
	{
		loadtestr.GET("/stop", func(c *gin.Context) {
			stopLoadTest(c, e)
		})
		loadtestr.GET("/:session", func(c *gin.Context) {
			startLoadTest(c, e)
		})
		loadtestr.GET("/stats", func(c *gin.Context) {
			stats(c, e)
		})
	}

	streamr := r.Group("stream")
	{
		streamr.GET("/demo/:session", func(c *gin.Context) {
			startTestStream(c, e)
		})
		streamr.GET("/stop", func(c *gin.Context) {
			stopStream(c, e)
		})
	}

	diskr := r.Group("disk")
	{
		diskr.GET("/:session", func(c *gin.Context) {
			saveToDisk(c, e)
		})
		diskr.GET("/stop", func(c *gin.Context) {
			stopDisk(c, e)
		})
	}

	return r.Run(port)
}
