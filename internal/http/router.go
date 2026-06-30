package http

import (
	"net/http"
	"os"

	"sangfor-log-search/internal/auth"
)

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	h := NewHandlers(deps)

	// Static files
	staticDir := "web/static"
	if envDir := os.Getenv("ZLOG_STATIC_DIR"); envDir != "" {
		staticDir = envDir
	}
	if _, err := os.Stat(staticDir); err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	}

	// Login: GET renders page, POST authenticates
	mux.HandleFunc("/login", h.LoginPage)
	mux.HandleFunc("/logout", h.Logout)

	mux.HandleFunc("/health", h.Health)

	mux.HandleFunc("/api/logs", deps.Sessions.RequireAuth(h.QueryAPI))
	mux.HandleFunc("/api/import", deps.Sessions.RequireAuth(h.ImportAPI))
	mux.HandleFunc("/api/jobs", deps.Sessions.RequireAuth(h.JobsAPI))
	mux.HandleFunc("/api/exports", deps.Sessions.RequireAuth(h.ExportAPI))
	mux.HandleFunc("/api/ip-ranges", deps.Sessions.RequireAuth(h.IPRangesAPI))

	mux.Handle("/", deps.Sessions.RequireAuth(dashboardHandler(deps)))
	mux.Handle("/tasks", deps.Sessions.RequireAuth(pageHandler("tasks")))
	mux.Handle("/ip-ranges", deps.Sessions.RequireAuth(pageHandler("ip_ranges")))
	mux.Handle("/settings", deps.Sessions.RequireAuth(pageHandler("settings")))

	return mux
}

func dashboardHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		pageHandlerWith("dashboard", map[string]any{
			"Page": "dashboard",
			"Title": "查询 — zlog",
		})(w, r)
	}
}

func pageHandler(name string) http.HandlerFunc {
	return pageHandlerWith(name, map[string]any{
		"Page": name,
	})
}

func pageHandlerWith(name string, extra map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := loadTemplate(name)
		if tmpl == nil {
			http.Error(w, "template not found", http.StatusInternalServerError)
			return
		}
		data := map[string]any{
			"Username": getUsername(r),
			"Page":     name,
			"Title":    "zlog",
		}
		for k, v := range extra {
			data[k] = v
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, data)
	}
}

func getUsername(r *http.Request) string {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}
