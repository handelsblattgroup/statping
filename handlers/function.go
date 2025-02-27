package handlers

import (
	"github.com/handelsblattgroup/statping/source"
	"github.com/handelsblattgroup/statping/types/core"
	"github.com/handelsblattgroup/statping/utils"
	"html/template"
	"net/http"
	"net/url"
)

var (
	basePath = "/"
)

func parseForm(r *http.Request) url.Values {
	r.ParseForm()
	return r.PostForm
}

var handlerFuncs = func(w http.ResponseWriter, r *http.Request) template.FuncMap {
	return template.FuncMap{
		"VERSION": func() string {
			return core.App.Version
		},
		"CoreApp": func() core.Core {
			c := *core.App
			if c.Name == "" {
				c.Name = "Statping"
			}
			return c
		},
		"USE_CDN": func() bool {
			return core.App.UseCdn.Bool
		},
		"USING_ASSETS": func() bool {
			return source.UsingAssets(utils.Directory)
		},
		"BasePath": func() string {
			return basePath
		},
		"Icon": func() string {
			return basePath
		},
	}
}
