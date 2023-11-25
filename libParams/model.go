package libParams

import (
	"image"
	"time"

	"github.com/hmmftg/image/font/opentype"
	"gopkg.in/natefinch/lumberjack.v2"
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

func LogRotate(logger *lumberjack.Logger) {
	go func() {
		for {
			t := time.Now()
			t = t.Truncate(time.Hour * 24)

			<-time.After(time.Duration(t.Hour()) * 24)
			logger.Rotate()
		}
	}()
}
