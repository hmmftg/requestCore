package libFiber

import (
	"net/http"

	"github.com/hmmftg/requestCore/libQuery"

	"github.com/gofiber/fiber/v2"
)

func InitContext(c any) FiberParser {
	return FiberParser{
		Ctx: c.(*fiber.Ctx),
	}
}

type FiberParser struct {
	Ctx *fiber.Ctx
}

func (c FiberParser) GetMethod() string {
	return c.Ctx.Method()
}

func (c FiberParser) GetPath() string {
	return c.Ctx.OriginalURL()
}
func (c FiberParser) GetHeader(target any) error {
	return c.Ctx.ReqHeaderParser(target)
}
func (c FiberParser) GetHeaderValue(name string) string {
	return c.Ctx.GetReqHeaders()[name]
}
func (c FiberParser) GetBody(target any) error {
	return c.Ctx.BodyParser(target)
}
func (c FiberParser) GetUrlQuery(target any) error {
	return c.Ctx.BodyParser(target)
}
func (c FiberParser) GetRawUrlQuery() string {
	return string(c.Ctx.Request().URI().QueryString())
}
func (c FiberParser) GetLocal(name string) any {
	return c.Ctx.Locals(name)
}
func (c FiberParser) GetLocalString(name string) string {
	value := c.Ctx.Locals(name)
	switch str := value.(type) {
	case string:
		return str
	}
	return ""
}
func (c FiberParser) GetUrlParam(name string) string {
	return c.Ctx.Params(name)
}
func (c FiberParser) GetUrlParams() map[string]string {
	return c.Ctx.AllParams()
}
func (c FiberParser) CheckUrlParam(name string) (string, bool) {
	value := c.Ctx.Params(name)
	return value, len(value) > 0
}

func (c FiberParser) SetLocal(name string, value any) {
	c.Ctx.Locals(name, value)
}

func (c FiberParser) GetArgs(args ...any) map[string]string {
	fiberArgs := map[string]string{
		"userId":   c.Ctx.Locals("userId").(string),
		"userName": c.Ctx.Locals("userName").(string),
		"appName":  c.Ctx.Locals("appName").(string),
		"action":   c.Ctx.Locals("action").(string),
		"bankCode": c.Ctx.Locals("bankCode").(string),
		"path":     c.Ctx.Route().Path,
	}

	for _, arg := range args {
		fiberArgs[arg.(string)] = c.Ctx.Params(arg.(string))
	}

	return fiberArgs
}

func (c FiberParser) ParseCommand(command, title string, request libQuery.RecordData, parser libQuery.FieldParser) string {

	if request.GetValueMap() == nil {
		return libQuery.ParseCommand(command, c.Ctx.Locals("userId").(string),
			c.Ctx.Locals("appName").(string),
			c.Ctx.Locals("action").(string),
			title,
			map[string]string{}, parser)
	}
	return libQuery.ParseCommand(command, c.Ctx.Locals("userId").(string),
		c.Ctx.Locals("appName").(string),
		c.Ctx.Locals("action").(string),
		title,
		request.GetValueMap(), parser)
}

func (c FiberParser) GetHttpHeader() http.Header {
	return ExtendMap(c.Ctx.GetReqHeaders())
}

func Fiber(handler any) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		handler.(func(c any))(c)
		return nil
	}
}

func ExtendMap(mp map[string]string) map[string][]string {
	newMap := map[string][]string{}
	for key, val := range mp {
		newMap[key] = []string{val}
	}
	return newMap
}
