package libParams

import (
	"image"

	"github.com/hmmftg/image/font/opentype"
)

type ParamsModel struct {
	Params map[string]string
}

type ParamsInterface interface {
	GetValue(name string) string
}

func (m ParamsModel) GetValue(name string) string {
	return m.Params[name]
}

type ParamInterface interface {
	GetFonts() map[string]opentype.Font
	GetImages() map[string]image.Image
	GetRoles() map[string]string
	GetParams() map[string]string
}
