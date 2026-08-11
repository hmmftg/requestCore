package libGin

import "github.com/gin-gonic/gin"

// GinParser wraps a Gin context to implement the webFramework parser interface.
type GinParser struct {
	Ctx *gin.Context
}
