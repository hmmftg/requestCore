package libFiber

import (
	"context"
	"net/http"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/gofiber/fiber/v2"
)

func InitContext(c *fiber.Ctx) FiberParser {
	return FiberParser{
		Ctx: c,
	}
}

func (c FiberParser) GetMethod() string {
	return c.Ctx.Method()
}

func (c FiberParser) GetPath() string {
	return c.Ctx.OriginalURL()
}
func (c FiberParser) GetHeader(target webFramework.HeaderInterface) error {
	targetPtr := target
	return c.Ctx.ReqHeaderParser(targetPtr)
}
func (c FiberParser) GetHeaderValue(name string) string {
	if len(c.Ctx.GetReqHeaders()[name]) > 0 {
		return c.Ctx.GetReqHeaders()[name][0]
	}
	return ""
}
func (c FiberParser) GetBody(target any) error {
	return c.Ctx.BodyParser(target)
}
func (c FiberParser) GetUri(target any) error {
	return c.Ctx.ParamsParser(target)
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

func (c FiberParser) SetReqHeader(name string, value string) {
	c.Ctx.Context().Request.Header.Set(name, value)
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

func (c FiberParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
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
	return c.Ctx.GetReqHeaders()
}

func (c FiberParser) SendJSONRespBody(status int, resp any) error {
	err := c.Ctx.JSON(resp)
	c.Ctx.Status(status)
	return err
}
func (c FiberParser) Next() error {
	return c.Ctx.Next()
}
func (c FiberParser) Abort() error {
	return c.Ctx.SendStatus(c.Ctx.Response().StatusCode())
}

func (c FiberParser) FormValue(name string) string {
	value := c.Ctx.FormValue(name, "")

	return value
}

func (c FiberParser) FormFile(
	formTagName, path string,
) error {
	fileHeader, fileErr := c.Ctx.FormFile(formTagName)
	if fileErr != nil {
		return fileErr
	}

	saveErr := c.Ctx.SaveFile(fileHeader, path)
	if saveErr != nil {
		return saveErr
	}

	return nil
}

const FiberCtxKey = "fiber.Ctx"

func Fiber(handler any) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		c.Context().SetUserValue(FiberCtxKey, c)
		handler.(func(c context.Context))(c.Context())
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
