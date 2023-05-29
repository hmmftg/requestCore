package libContext

import (
	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
)

func InitContext(c any) webFramework.WebFramework {
	w := webFramework.WebFramework{
		Ctx: c,
	}
	switch c.(type) {
	case *fiber.Ctx:
		w.Parser = libFiber.InitContext(c)
	case *gin.Context:
		w.Parser = libGin.InitContext(c)
	}
	return w
}

func InitContextWithHandler(c any, handler response.ResponseHandler) webFramework.WebFramework {
	w := webFramework.WebFramework{
		Ctx: c,
		//Handler: handler,
	}
	switch c.(type) {
	case *fiber.Ctx:
		w.Parser = libFiber.InitContext(c)
	case *gin.Context:
		w.Parser = libGin.InitContext(c)
	}
	return w
}
