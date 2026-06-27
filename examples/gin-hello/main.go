package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libContext"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.GET("/health", func(c *gin.Context) {
		wf := libContext.InitContextNoAuditTrail(c)
		if err := wf.Parser.SendJSONRespBody(http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	})

	router.GET("/users/:id", func(c *gin.Context) {
		wf := libContext.InitContextNoAuditTrail(c)
		id := c.Param("id")
		if err := wf.Parser.SendJSONRespBody(http.StatusOK, map[string]string{"id": id}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	})

	router.POST("/echo", func(c *gin.Context) {
		wf := libContext.InitContextNoAuditTrail(c)
		var body struct {
			Message string `json:"message"`
		}
		if err := wf.Parser.GetBody(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := wf.Parser.SendJSONRespBody(http.StatusOK, body); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	})

	log.Println("gin-hello listening on :8081")
	log.Fatal(router.Run(":8081"))
}
