package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"sangfor-log-search/internal/auth"
	"sangfor-log-search/internal/config"
	"sangfor-log-search/internal/exporter"
	"sangfor-log-search/internal/importer"
	"sangfor-log-search/internal/ipmeta"
	"sangfor-log-search/internal/jobs"
	"sangfor-log-search/internal/model"
	"sangfor-log-search/internal/query"
	"sangfor-log-search/internal/store"
)

type Deps struct {
	Cfg       config.Config
	Store     *store.ClickHouseStore
	Importer  *importer.Importer
	Exporter  *exporter.Exporter
	JobStore  jobs.JobStore
	Resolver  *ipmeta.Resolver
	Sessions  *auth.SessionManager
}

type Handlers struct {
	deps Deps
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{deps: deps}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	h.deps.Sessions.LoginHandler(h.deps.Cfg.Auth.AdminUsername, h.deps.Cfg.Auth.AdminPasswordHash)(w, r)
}

func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl := loadTemplate("login")
		if tmpl == nil {
			http.Error(w, "template not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, map[string]any{
			"Page":  "login",
			"Title": "登录 — zlog",
		})
		return
	}
	h.Login(w, r)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.deps.Sessions.LogoutHandler()(w, r)
}

func (h *Handlers) parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = parseFlexibleTime(startStr, true)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("start: %w", err)
		}
	}
	if endStr != "" {
		end, err = parseFlexibleTime(endStr, false)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("end: %w", err)
		}
	}

	if h.deps.Cfg.Query.RequireTimeRange && (start.IsZero() || end.IsZero()) {
		return time.Time{}, time.Time{}, fmt.Errorf("time range is required")
	}

	if !start.IsZero() && !end.IsZero() {
		days := end.Sub(start).Hours() / 24
		if days > float64(h.deps.Cfg.Query.MaxRangeDays) {
			return time.Time{}, time.Time{}, fmt.Errorf("time range exceeds %d days", h.deps.Cfg.Query.MaxRangeDays)
		}
	}

	return start, end, nil
}

func parseFlexibleTime(s string, isStart bool) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", "20060102"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			if isStart && layout != "2006-01-02 15:04:05" {
				return t, nil
			}
			if !isStart && layout == "2006-01-02" {
				return t.Add(24*time.Hour - time.Second), nil
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time: %s", s)
}

func (h *Handlers) buildFilter(r *http.Request) (query.LogFilter, error) {
	start, end, err := h.parseTimeRange(r)
	if err != nil {
		return query.LogFilter{}, err
	}

	return query.LogFilter{
		Start:    start,
		End:      end,
		IP:       r.URL.Query().Get("ip"),
		IPField:  r.URL.Query().Get("field"),
		DeviceIP: r.URL.Query().Get("device"),
		FilePart: r.URL.Query().Get("file"),
		NatType:  r.URL.Query().Get("nat_type"),
		Protocol: r.URL.Query().Get("protocol"),
		Keyword:  r.URL.Query().Get("keyword"),
	}, nil
}

func (h *Handlers) QueryAPI(w http.ResponseWriter, r *http.Request) {
	f, err := h.buildFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	pageSize := h.deps.Cfg.Query.DefaultPageSize
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= h.deps.Cfg.Query.MaxPageSize {
			pageSize = v
		}
	}
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	offset := (page - 1) * pageSize

	rows, err := h.deps.Store.Query(r.Context(), f, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}

	total, err := h.deps.Store.Count(r.Context(), f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, queryResult{
		Rows:  toLogViews(rows),
		Total: total,
		Page:  page,
		PageSize: pageSize,
	})
}

func (h *Handlers) ImportAPI(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = h.deps.Cfg.Paths.LogDir
	}

	force := r.URL.Query().Get("force") == "true"

	go func() {
		_, _ = h.deps.Importer.ImportDirectory(r.Context(), dir)
		_ = force
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{"status": "import started", "dir": dir})
}

func (h *Handlers) JobsAPI(w http.ResponseWriter, r *http.Request) {
	jobsList, err := h.deps.JobStore.List(r.Context(), 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobsList})
}

func (h *Handlers) ExportAPI(w http.ResponseWriter, r *http.Request) {
	f, err := h.buildFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	jobID, err := h.deps.Exporter.EnqueueCSV(r.Context(), f, format)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
}

func (h *Handlers) IPRangesAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		customPath := os.Getenv("ZLOG_IPMETA_CUSTOM")
		if customPath == "" {
			customPath = "/opt/zlog/custom-ranges.yaml"
		}
		entries, err := readCustomRanges(customPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ranges": entries})

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid form"})
			return
		}
		cidr := r.FormValue("cidr")
		name := r.FormValue("name")
		country := r.FormValue("country")
		if cidr == "" || name == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "cidr and name required"})
			return
		}

		customPath := os.Getenv("ZLOG_IPMETA_CUSTOM")
		if customPath == "" {
			customPath = "/opt/zlog/custom-ranges.yaml"
		}
		if err := appendCustomRange(customPath, cidr, name, country); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "saved"})

	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "cidr required"})
			return
		}
		customPath := os.Getenv("ZLOG_IPMETA_CUSTOM")
		if customPath == "" {
			customPath = "/opt/zlog/custom-ranges.yaml"
		}
		if err := removeCustomRange(customPath, cidr); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

type logView struct {
	Ts             string `json:"ts"`
	DeviceIP       string `json:"device_ip"`
	LogType        string `json:"log_type"`
	NatType        string `json:"nat_type"`
	SrcIP          string `json:"src_ip"`
	SrcPort        uint16 `json:"src_port"`
	DstIP          string `json:"dst_ip"`
	DstCountry     string `json:"dst_country"`
	DstPort        uint16 `json:"dst_port"`
	Protocol       uint8  `json:"protocol"`
	TranslatedIP   string `json:"translated_ip"`
	TranslatedPort uint16 `json:"translated_port"`
	SourceFile     string `json:"source_file"`
	LineNo         uint32 `json:"line_no"`
}

func toLogViews(rows []model.LogRow) []logView {
	views := make([]logView, 0, len(rows))
	for _, r := range rows {
		views = append(views, logView{
			Ts:             r.Ts.Format("2006-01-02 15:04:05"),
			DeviceIP:       r.DeviceIP.String(),
			LogType:        r.LogType,
			NatType:        r.NatType,
			SrcIP:          r.SrcIP.String(),
			SrcPort:        r.SrcPort,
			DstIP:          r.DstIP.String(),
			DstCountry:     r.DstCountry,
			DstPort:        r.DstPort,
			Protocol:       r.Protocol,
			TranslatedIP:   r.TranslatedIP.String(),
			TranslatedPort: r.TranslatedPort,
			SourceFile:     r.SourceFile,
			LineNo:         r.LineNo,
		})
	}
	return views
}

type queryResult struct {
	Rows     []logView `json:"rows"`
	Total    uint64    `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ===== Custom IP ranges file CRUD =====

type customRangeView struct {
	CIDR    string `json:"cidr" yaml:"cidr"`
	Name    string `json:"name" yaml:"name"`
	Country string `json:"country" yaml:"country"`
}

func readCustomRanges(path string) ([]customRangeView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []customRangeView{}, nil
		}
		return nil, fmt.Errorf("read custom ranges: %w", err)
	}
	var entries []customRangeView
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse custom ranges: %w", err)
	}
	return entries, nil
}

func appendCustomRange(path, cidr, name, country string) error {
	entries, _ := readCustomRanges(path)
	for _, e := range entries {
		if e.CIDR == cidr {
			return fmt.Errorf("CIDR %s already exists", cidr)
		}
	}
	entries = append(entries, customRangeView{CIDR: cidr, Name: name, Country: country})
	return writeCustomRanges(path, entries)
}

func removeCustomRange(path, cidr string) error {
	entries, err := readCustomRanges(path)
	if err != nil {
		return err
	}
	var out []customRangeView
	for _, e := range entries {
		if e.CIDR != cidr {
			out = append(out, e)
		}
	}
	return writeCustomRanges(path, out)
}

func writeCustomRanges(path string, entries []customRangeView) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
