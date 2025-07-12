package swagger

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

func FiberHandler(docs string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Allow all origins
		c.Response().Header.Add("Access-Control-Allow-Origin", "*")
		c.Response().Header.Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Response().Header.Add("Access-Control-Allow-Headers", "Content-Type")

		switch c.Params("file") {
		case "/", IndexFileName:
			_, err := c.WriteString(indexTempl)
			if err != nil {
				log.Println("Write index.html error", err)
			}
			return err

		case DocJSON:
			c.Response().Header.Add("Content-Type", "application/json")
			_, err := c.WriteString(docs)
			if err != nil {
				log.Println("Write v1.json error", err)
			}
			return err
		}

		err := c.SendStatus(http.StatusNotFound)
		if err != nil {
			log.Println("Write 404.html error", err)
		}
		return err
	}
}
