package swagger

import "html/template"

type swaggerConfig struct {
	URL                      string
	Base                     string
	Name                     string
	DocExpansion             string
	Title                    string
	Oauth2RedirectURL        template.JS
	DefaultModelsExpandDepth int
	DeepLinking              bool
	PersistAuthorization     bool
	Oauth2DefaultClientID    string
}

func defaultConfig(title, base, name string) swaggerConfig {
	return swaggerConfig{
		Base:                     base,
		Name:                     name,
		URL:                      "doc.json",
		DeepLinking:              false,
		DocExpansion:             "list",
		DefaultModelsExpandDepth: -1,
		Oauth2RedirectURL: "`${window.location.protocol}//${window.location.host}$" +
			"{window.location.pathname.split('/').slice(0, window.location.pathname.split('/').length - 1).join('/')}" +
			"/oauth2-redirect.html`",
		Title:                 title,
		PersistAuthorization:  true,
		Oauth2DefaultClientID: "ClientID",
	}
}
