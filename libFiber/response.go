package libFiber

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/webFramework"
)

type ContextInitiator interface {
	InitContext(c *fiber.Ctx) webFramework.WebFramework
	Respond(int, int, string, any, bool, webFramework.WebFramework)
}

func ErrorHandler(path, title string, handler ContextInitiator) fiber.ErrorHandler {
	log.Println("ErrorHandler: ", path, title)
	return func(c *fiber.Ctx, err error) error {
		w := handler.InitContext(c)
		log.Println(path, title, "ErrorHandler", err)
		switch err := err.(type) {
		case *fiber.Error:
			switch err.Code {
			case 404:
				handler.Respond(http.StatusNotFound, 1, "PAGE_NOT_FOUND", err, true, w)
				return nil
			}
			log.Println("Fiber Error", err.Code, err.Message)
			handler.Respond(http.StatusInternalServerError, 1, "INTERNAL_ERROR", err, true, w)
		default:
			if c.Locals("LastError") != nil {
				log.Println("LocalError", err)
				handler.Respond(http.StatusInternalServerError, 1, c.Locals("LastError").(string), nil, true, w)
				return nil
			}
			log.Println("Unknown", err)
			handler.Respond(http.StatusInternalServerError, 1, "INTERNAL_ERROR", nil, true, w)
			return nil
		}
		log.Println("Unknown", err)
		handler.Respond(http.StatusInternalServerError, 1, "INTERNAL_ERROR", nil, true, w)
		return nil
	}
}
