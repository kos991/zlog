package http

import (
	"net/http"

	"sangfor-log-search/internal/auth"
)

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	h := NewHandlers(deps)

	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/login", h.Login)
	mux.HandleFunc("/logout", h.Logout)

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
		pageHandler("dashboard")(w, r)
	}
}

func pageHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := loadTemplate(name)
		if tmpl == nil {
			http.Error(w, "template not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, map[string]any{
			"Username": getUsername(r),
		})
	}
}

func getUsername(r *http.Request) string {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}
