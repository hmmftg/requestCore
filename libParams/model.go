package libParams

type ParamsModel struct {
	Params map[string]string
}

type ParamsInterface interface {
	GetValue(name string) string
}

func (m ParamsModel) GetValue(name string) string {
	return m.Params[name]
}
