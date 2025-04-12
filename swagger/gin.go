package swagger

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/swaggo/swag/v2"
)

func GinHandler(base, name string, spec *swag.Spec) gin.HandlerFunc {
	index, err := template.New("swagger_index.html").Parse(indexTempl)
	if err != nil {
		log.Println("get template", err)
		return nil
	}
	return func(c *gin.Context) {
		start := time.Now()
		status := http.StatusOK
		finalize := libContext.AddWebHandlerLogs(c, "Swagger", "swagger-handler")
		defer finalize(start, status)
		if c.Request.Method != http.MethodGet {
			status = http.StatusMethodNotAllowed
			c.AbortWithStatus(status)
			return
		}

		matches := matcher.FindStringSubmatch(c.Request.RequestURI)

		if len(matches) != 3 {
			log.Println("not found", c.Request.RequestURI)
			status = http.StatusOK
			c.String(status, "NOT FOUND, url: %s", c.Request.RequestURI)
			return
		}

		path := matches[2]

		// Allow all origins
		c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Writer.Header().Add("Access-Control-Allow-Headers", "Content-Type")

		switch path {
		case "/", IndexFileName:
			err := index.Execute(c.Writer, defaultConfig(spec.Title, base, name))
			if err != nil {
				log.Println("Write index.html error", err)
				status = http.StatusInternalServerError
				c.AbortWithStatus(status)
				return
			}
			return

		case DocJSON:
			c.Writer.Header().Add("Content-Type", "application/json")
			doc, errRead := swag.ReadDoc(spec.InfoInstanceName)
			if errRead != nil {
				log.Println("Read Doc error", errRead)
				status = http.StatusInternalServerError
				c.AbortWithStatus(status)
				return
			}
			_, err := c.Writer.WriteString(doc)
			if err != nil {
				log.Println("Write v1.json error", err)
				status = http.StatusInternalServerError
				c.AbortWithStatus(status)
				return
			}
			return
		case CSS, Fav16, Fav32, Bundle, Preset:
			swaggerPath := os.Getenv("SWAGGER")
			c.File(fmt.Sprintf("%s/%s", swaggerPath, path))
			return
		}
		log.Println("not found", path)
		status = http.StatusNotFound
		c.String(status, "NOT FOUND, path: %s, url: %s", path, c.Request.RequestURI)
	}
}
