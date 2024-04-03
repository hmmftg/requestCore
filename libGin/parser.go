package libGin

import (
	"context"
	"net/http"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/gin-gonic/gin"
)

func InitContext(c any) GinParser {
	return GinParser{Ctx: c.(*gin.Context)}
}

func (c GinParser) GetMethod() string {
	return c.Ctx.Request.Method
}

func (c GinParser) GetPath() string {
	return c.Ctx.FullPath()
}

func (c GinParser) GetHeader(target webFramework.HeaderInterface) error {
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
func (c GinParser) GetUri(target any) error {
	return c.Ctx.ShouldBindUri(target)
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

func (c GinParser) SetReqHeader(name string, value string) {
	c.Ctx.Request.Header.Set(name, value)
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

func (c GinParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	return libQuery.ParseCommand(command,
		c.Ctx.GetString("userId"),
		c.Ctx.GetString("appName"),
		c.Ctx.GetString("action"),
		c.Ctx.GetString(title), request.GetValueMap(), parser)
}

func (c GinParser) GetHttpHeader() http.Header {
	return c.Ctx.Request.Header
}

func (c GinParser) SendJSONRespBody(status int, resp any) error {
	c.Ctx.JSON(status, resp)
	return nil
}
func (c GinParser) Next() error {
	c.Ctx.Next()
	return nil
}
func (c GinParser) Abort() error {
	c.Ctx.Abort()
	return nil
}

func (c GinParser) FormValue(name string) string {
	value := c.Ctx.Request.FormValue(name)

	return value
}

func (c GinParser) SaveFile(
	formTagName, path string,
) error {
	file, fileHeaders, fileErr := c.Ctx.Request.FormFile(formTagName)
	if fileErr != nil {
		return fileErr
	}
	defer file.Close()

	saveErr := c.Ctx.SaveUploadedFile(fileHeaders, path)
	if saveErr != nil {
		return saveErr
	}

	return nil
}

func Gin(handler any) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.(func(c context.Context))(c)
	}
}
