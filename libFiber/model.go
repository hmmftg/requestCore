package libFiber

import "github.com/gofiber/fiber/v2"

// FiberParser implements the webFramework.RequestParser interface for Fiber.
type FiberParser struct {
	Ctx *fiber.Ctx
}
