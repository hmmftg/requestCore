package libGin

import (
	"net/http"

	"github.com/hmmftg/requestCore/libQuery"

	"github.com/gin-gonic/gin"
)

func InitContext(c any) GinParser {
	return GinParser{Ctx: c.(*gin.Context)}
}

type GinParser struct {
	Ctx *gin.Context
}

func (c GinParser) GetMethod() string {
	return c.Ctx.Request.Method
}

func (c GinParser) GetPath() string {
	return c.Ctx.FullPath()
}

func (c GinParser) GetHeader(target any) error {
	return c.Ctx.ShouldBindHeader(target)
}
func (c GinParser) GetHeaderValue(name string) string {
	return c.Ctx.Request.Header.Get(name)
}
func (c GinParser) GetRawUrlQuery() string {
	return c.Ctx.Request.URL.RawQuery
}
func (c GinParser) GetBody(target any) error {
	return c.Ctx.ShouldBindJSON(target)
}
func (c GinParser) GetUrlQuery(target any) error {
	return c.Ctx.ShouldBindQuery(target)
}
func (c GinParser) GetLocal(name string) any {
	value, _ := c.Ctx.Get(name)
	return value
}
func (c GinParser) GetLocalString(name string) string {
	return c.Ctx.GetString(name)
}
func (c GinParser) GetUrlParam(name string) string {
	return c.Ctx.Params.ByName(name)
}
func (c GinParser) GetUrlParams() map[string]string {
	ginParams := c.Ctx.Params
	result := make(map[string]string, 0)
	for _, param := range ginParams {
		result[param.Key] = param.Value
	}
	return result
}
func (c GinParser) CheckUrlParam(name string) (string, bool) {
	return c.Ctx.Params.Get(name)
}

func (c GinParser) SetLocal(name string, value any) {
	c.Ctx.Set(name, value)
}

func (c GinParser) GetArgs(args ...any) map[string]string {
	ginArgs := map[string]string{
		"userId":   c.Ctx.GetString("userId"),
		"appName":  c.Ctx.GetString("appName"),
		"action":   c.Ctx.GetString("action"),
		"bankCode": c.Ctx.GetHeader("Bank-Code"),
	}

	for _, arg := range args {
		ginArgs[arg.(string)] = c.Ctx.Param(arg.(string))
	}

	return ginArgs
}

func (c GinParser) ParseCommand(command, title string, request libQuery.RecordData, parser libQuery.FieldParser) string {
	return libQuery.ParseCommand(command,
		c.Ctx.GetString("userId"),
		c.Ctx.GetString("appName"),
		c.Ctx.GetString("action"),
		c.Ctx.GetString(title), request.GetValueMap(), parser)
}

func (c GinParser) GetHttpHeader() http.Header {
	return c.Ctx.Request.Header
}

func Gin(handler any) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.(func(c any))(c)
	}
}
