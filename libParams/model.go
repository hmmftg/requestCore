package libParams

import (
	"fmt"
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
	// ticker := time.NewTicker(time.Second * 1)
	ticker := time.NewTicker(time.Hour)
	go func() {
		for {
			<-ticker.C
			h, m, s := time.Now().Clock()
			if h == 0 {
				//if m == 0 && h == 0 {
				fmt.Println("========= performe log-rotate", h, m, s)
				err := logger.Rotate()
				if err != nil {
					fmt.Println("========= error in log-rotate", err)
				}
			} else {
				fmt.Println("log-heart-beat", h, m, s)
			}
		}
	}()
}
