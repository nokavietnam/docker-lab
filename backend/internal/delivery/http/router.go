package http

import "github.com/gin-gonic/gin"

func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	// v1 := r.Group("/api/v1") {
	//
	// }

	return r
}
