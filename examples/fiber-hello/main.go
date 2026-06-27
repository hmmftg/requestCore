package main

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libContext"
)

func main() {
	app := fiber.New()

	app.Get("/health", func(c *fiber.Ctx) error {
		wf := libContext.InitContextNoAuditTrail(c)
		return wf.Parser.SendJSONRespBody(http.StatusOK, map[string]string{"status": "ok"})
	})

	app.Get("/users/:id", func(c *fiber.Ctx) error {
		wf := libContext.InitContextNoAuditTrail(c)
		id := c.Params("id")
		return wf.Parser.SendJSONRespBody(http.StatusOK, map[string]string{"id": id})
	})

	app.Post("/echo", func(c *fiber.Ctx) error {
		wf := libContext.InitContextNoAuditTrail(c)
		var body struct {
			Message string `json:"message"`
		}
		if err := wf.Parser.GetBody(&body); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return wf.Parser.SendJSONRespBody(http.StatusOK, body)
	})

	log.Println("fiber-hello listening on :8082")
	log.Fatal(app.Listen(":8082"))
}
