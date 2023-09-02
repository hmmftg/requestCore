package libFiber

import (
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

func ErrorHandler(path, title string, respondHandler func(int, int, string, any, bool, any)) fiber.ErrorHandler {
	log.Println("ErrorHandler: ", path, title)
	return func(c *fiber.Ctx, err error) error {
		log.Println(path, title, "ErrorHandler", err)
		switch err := err.(type) {
		case *fiber.Error:
			switch err.Code {
			case 404:
				respondHandler(http.StatusNotFound, 1, "PAGE_NOT_FOUND", err, true, c)
				return nil
			}
			log.Println("Fiber Error", err.Code, err.Message)
			respondHandler(http.StatusInternalServerError, 1, "INTERNAL_ERROR", err, true, c)
		default:
			if c.Locals("LastError") != nil {
				log.Println("LocalError", err)
				respondHandler(http.StatusInternalServerError, 1, c.Locals("LastError").(string), nil, true, c)
				return nil
			}
			log.Println("Unknown", err)
			respondHandler(http.StatusInternalServerError, 1, "INTERNAL_ERROR", nil, true, c)
			return nil
		}
		log.Println("Unknown", err)
		respondHandler(http.StatusInternalServerError, 1, "INTERNAL_ERROR", nil, true, c)
		return nil
	}
}
