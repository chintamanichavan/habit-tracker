package httpx

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"
)

type templateSet struct {
	tmpls *template.Template
}

func loadTemplates(templatesFS fs.FS) (*templateSet, error) {
	funcs := template.FuncMap{
		"fmtDate": func(t time.Time) string {
			return t.Format("Mon Jan 2")
		},
		"fmtISO": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"title": func(s string) string {
			if s == "" {
				return ""
			}
			return string(s[0]-32) + s[1:]
		},
	}
	t, err := template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &templateSet{tmpls: t}, nil
}

func (ts *templateSet) renderPage(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ts.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
	}
}

func (ts *templateSet) renderFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ts.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
	}
}

// staticFS is embedded by the parent package and passed in.
type Server struct {
	store     storeIface
	loc       *time.Location
	tmpls     *templateSet
	staticDir http.FileSystem
}

// embeddedAssets is wired by NewServer; declared in handlers.go via embed.
var _ = embed.FS{}
