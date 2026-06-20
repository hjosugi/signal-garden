package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/hjosugi/signal-garden/internal/ai"
	"github.com/hjosugi/signal-garden/internal/collector"
	"github.com/hjosugi/signal-garden/internal/config"
	"github.com/hjosugi/signal-garden/internal/search"
	"github.com/hjosugi/signal-garden/internal/store"
)

type Server struct {
	app       config.App
	site      config.Site
	feeds     []config.Feed
	store     *store.JSONStore
	collector *collector.Collector
	ollama    *ai.Ollama
	hub       *Hub
	logger    *slog.Logger
	templates *template.Template
	mux       *http.ServeMux
}

type basePage struct {
	Site         config.Site
	CurrentPath  string
	Year         int
	ViewerCount  int
	InboxLocked  bool
	EmbeddingsOn bool
	SummariesOn  bool
}

type homePage struct {
	basePage
	Featured []config.Project
	Others   []config.Project
}

type inboxPage struct {
	basePage
	Query      string
	Tag        string
	Source     string
	Results    []search.Result
	Facets     search.Facets
	Total      int
	Running    bool
	LastReport any
	Statuses   any
	Semantic   bool
}

type unlockPage struct {
	basePage
	Error string
	Next  string
}

func NewServer(app config.App, site config.Site, feeds []config.Feed, dataStore *store.JSONStore, col *collector.Collector, ollama *ai.Ollama, hub *Hub, logger *slog.Logger) (*Server, error) {
	funcs := template.FuncMap{
		"join":  strings.Join,
		"since": since,
		"date": func(t time.Time) string {
			if t.IsZero() {
				return "unknown"
			}
			return t.Format("2006-01-02")
		},
		"score":       func(v float64) string { return fmt.Sprintf("%.2f", v) },
		"lower":       strings.ToLower,
		"hasPrefix":   strings.HasPrefix,
		"queryEscape": url.QueryEscape,
	}
	tmpl, err := template.New("pages").Funcs(funcs).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		app:       app,
		site:      site,
		feeds:     append([]config.Feed(nil), feeds...),
		store:     dataStore,
		collector: col,
		ollama:    ollama,
		hub:       hub,
		logger:    logger,
		templates: tmpl,
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.recoverer(s.securityHeaders(s.requestLogger(s.mux)))
}

func (s *Server) routes() {
	staticFS, _ := fs.Sub(assets, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	s.mux.HandleFunc("GET /", s.handleHome)
	s.mux.HandleFunc("GET /inbox", s.requireInbox(s.handleInbox))
	s.mux.HandleFunc("GET /unlock", s.handleUnlock)
	s.mux.HandleFunc("POST /unlock", s.handleUnlock)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/search", s.requireInbox(s.handleSearchAPI))
	s.mux.HandleFunc("GET /api/status", s.requireInbox(s.handleStatusAPI))
	s.mux.HandleFunc("POST /api/refresh", s.handleRefreshAPI)
	s.mux.HandleFunc("GET /events", s.handleEvents)
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /robots.txt", s.handleRobots)
	s.mux.HandleFunc("GET /sitemap.xml", s.handleSitemap)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	featured := make([]config.Project, 0)
	others := make([]config.Project, 0)
	for _, project := range s.site.Projects {
		if project.Featured {
			featured = append(featured, project)
		} else {
			others = append(others, project)
		}
	}
	data := homePage{basePage: s.base(r, "/"), Featured: featured, Others: others}
	s.render(w, http.StatusOK, "home", data)
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := clampString(r.URL.Query().Get("q"), 300)
	tag := clampString(r.URL.Query().Get("tag"), 80)
	source := clampString(r.URL.Query().Get("source"), 120)
	limit := parseLimit(r.URL.Query().Get("limit"), 50)
	results, semantic := s.runSearch(r.Context(), query, tag, source, limit)
	items := s.store.All()
	running, last, statuses := s.collector.Snapshot()
	data := inboxPage{
		basePage:   s.base(r, "/inbox"),
		Query:      query,
		Tag:        tag,
		Source:     source,
		Results:    results,
		Facets:     search.BuildFacets(items),
		Total:      len(items),
		Running:    running,
		LastReport: last,
		Statuses:   statuses,
		Semantic:   semantic,
	}
	s.render(w, http.StatusOK, "inbox", data)
}

func (s *Server) handleSearchAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := clampString(r.URL.Query().Get("q"), 300)
	tag := clampString(r.URL.Query().Get("tag"), 80)
	source := clampString(r.URL.Query().Get("source"), 120)
	limit := parseLimit(r.URL.Query().Get("limit"), 50)
	results, semantic := s.runSearch(r.Context(), query, tag, source, limit)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"query": query, "tag": tag, "source": source, "semantic": semantic, "results": results,
	})
}

func (s *Server) runSearch(ctx context.Context, query, tag, source string, limit int) ([]search.Result, bool) {
	var vector []float64
	semantic := false
	if strings.TrimSpace(query) != "" && s.app.EnableEmbeddings && s.ollama.CanEmbed() {
		callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		vectors, err := s.ollama.Embed(callCtx, []string{query})
		cancel()
		if err == nil && len(vectors) == 1 {
			vector = vectors[0]
			semantic = true
		} else if err != nil {
			s.logger.Debug("semantic query fallback", "error", err)
		}
	}
	return search.Run(s.store.All(), search.Query{
		Text: query, Tag: tag, Source: source, Limit: limit, QueryVector: vector,
	}, time.Now().UTC()), semantic
}

func (s *Server) handleStatusAPI(w http.ResponseWriter, _ *http.Request) {
	running, report, statuses := s.collector.Snapshot()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"running":     running,
		"items":       s.store.Count(),
		"last_report": report,
		"sources":     statuses,
		"viewers":     s.hub.Count(),
	})
}

func (s *Server) handleRefreshAPI(w http.ResponseWriter, r *http.Request) {
	if s.app.AdminToken == "" {
		http.Error(w, "refresh endpoint is disabled", http.StatusNotFound)
		return
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.app.AdminToken)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.collector.RefreshAsync(context.Background()); err != nil {
		if errors.Is(err, collector.ErrAlreadyRunning) {
			s.writeJSON(w, http.StatusConflict, map[string]any{"status": "already_running"})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, http.StatusAccepted, map[string]any{"status": "started"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	id, events, count := s.hub.Register()
	defer func() {
		remaining := s.hub.Unregister(id)
		s.hub.Publish("viewer_count", map[string]int{"count": remaining})
	}()
	s.hub.Publish("viewer_count", map[string]int{"count": count})
	writeSSE(w, Event{Type: "viewer_count", Data: mustJSON(map[string]int{"count": count}), Timestamp: time.Now().UTC()})
	flusher.Flush()

	private := s.inboxAuthorized(r)
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			if !private && event.Type != "viewer_count" {
				continue
			}
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.URL.Query().Get("next"))
	if next == "" {
		next = "/inbox"
	}
	if s.app.InboxToken == "" || s.inboxAuthorized(r) {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	data := unlockPage{basePage: s.base(r, "/unlock"), Next: next}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			data.Error = "Invalid form."
			s.render(w, http.StatusBadRequest, "unlock", data)
			return
		}
		provided := r.FormValue("token")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.app.InboxToken)) != 1 {
			data.Error = "Token is not valid."
			s.render(w, http.StatusUnauthorized, "unlock", data)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: "sg_inbox", Value: s.sessionValue(), Path: "/", HttpOnly: true,
			Secure: requestIsSecure(r), SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 30,
		})
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	s.render(w, http.StatusOK, "unlock", data)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "sg_inbox", Value: "", Path: "/", HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) requireInbox(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.inboxAuthorized(r) {
			next(w, r)
			return
		}
		target := "/unlock?next=" + url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func (s *Server) inboxAuthorized(r *http.Request) bool {
	if s.app.InboxToken == "" {
		return true
	}
	cookie, err := r.Cookie("sg_inbox")
	if err != nil {
		return false
	}
	expected := s.sessionValue()
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) == 1
}

func (s *Server) sessionValue() string {
	mac := hmac.New(sha256.New, []byte(s.app.InboxToken))
	mac.Write([]byte("signal-garden-inbox-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": s.store.Count()})
}

func (s *Server) handleRobots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "User-agent: *\nAllow: /\nDisallow: /inbox\nDisallow: /api/\nDisallow: /unlock\n")
}

func (s *Server) handleSitemap(w http.ResponseWriter, _ *http.Request) {
	if s.app.PublicBaseURL == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>%s/</loc></url></urlset>`, template.HTMLEscapeString(s.app.PublicBaseURL))
}

func (s *Server) base(r *http.Request, current string) basePage {
	return basePage{
		Site: s.site, CurrentPath: current, Year: time.Now().Year(), ViewerCount: s.hub.Count(),
		InboxLocked:  s.app.InboxToken != "" && !s.inboxAuthorized(r),
		EmbeddingsOn: s.app.EnableEmbeddings,
		SummariesOn:  s.app.EnableLLMSummary,
	}
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("render template", "template", name, "error", err)
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(data); err != nil {
		s.logger.Error("write JSON", "error", err)
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; connect-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic", "value", recovered, "stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != http.StatusOK {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func writeSSE(w http.ResponseWriter, event Event) {
	payload, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload)
}

func mustJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func since(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func parseLimit(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 200 {
		return fallback
	}
	return parsed
}

func clampString(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}

func safeNext(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return ""
	}
	return value
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
