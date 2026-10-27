package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	SiteName                string   `json:"site_name"`
	EmailDisplayName        string   `json:"email_display_name"`
	IntervalSeconds         int      `json:"interval_seconds"`
	TimeoutSeconds          int      `json:"timeout_seconds"`
	FailuresBeforeAlert     int      `json:"failures_before_alert"`
	SuccessesBeforeRecovery int      `json:"successes_before_recovery"`
	CertificateWarningDays  int      `json:"certificate_warning_days"`
	Targets                 []Target `json:"targets"`
}

type Target struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	ExpectedStatus []int  `json:"expected_status"`
	Contains       string `json:"contains,omitempty"`
}

type TargetState struct {
	Name              string    `json:"name"`
	URL               string    `json:"url"`
	Status            string    `json:"status"`
	ConsecutiveFail   int       `json:"consecutive_failures"`
	ConsecutiveOK     int       `json:"consecutive_successes"`
	LastChecked       time.Time `json:"last_checked"`
	LastChanged       time.Time `json:"last_changed"`
	LatencyMS         int64     `json:"latency_ms"`
	HTTPStatus        int       `json:"http_status,omitempty"`
	Detail            string    `json:"detail"`
	CertExpires       time.Time `json:"certificate_expires,omitempty"`
	CertWarnedFor     string    `json:"certificate_warning_for,omitempty"`
	StateAlertPending string    `json:"state_alert_pending,omitempty"`
	CertAlertPending  string    `json:"certificate_alert_pending,omitempty"`
	DownAlertSent     bool      `json:"down_alert_sent,omitempty"`
}

type SMTPConfig struct {
	Host, Port, TLSMode, Username, Password, From string
	To                                            []string
}

type App struct {
	cfg       Config
	smtp      SMTPConfig
	client    *http.Client
	mu        sync.RWMutex
	states    map[string]*TargetState
	stateFile string
}

var dashboard = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.SiteName}} monitor</title><style>
body{font-family:system-ui,sans-serif;background:#111827;color:#e5e7eb;margin:0;padding:2rem}main{max-width:960px;margin:auto}h1{font-size:1.5rem}.grid{display:grid;gap:1rem;grid-template-columns:repeat(auto-fit,minmax(260px,1fr))}.card{background:#1f2937;border:1px solid #374151;border-radius:12px;padding:1rem}.up{color:#34d399}.down{color:#fb7185}.pending,.unknown{color:#fbbf24}.muted{color:#9ca3af;font-size:.9rem}code{overflow-wrap:anywhere}</style></head>
<body><main><h1>{{.SiteName}}</h1><p class="muted">Independent external checks. Updated {{.Now}}</p><div class="grid">
{{range .States}}<section class="card"><strong>{{.Name}}</strong><p class="{{.Status}}">{{.Status}}</p><code>{{.URL}}</code><p>{{.Detail}}</p><p class="muted">{{.LatencyMS}} ms · {{.LastChecked}}</p></section>{{end}}
</div></main></body></html>`))

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local HTTP health endpoint")
	flag.Parse()
	if *healthcheck {
		c := http.Client{Timeout: 3 * time.Second}
		r, err := c.Get("http://127.0.0.1:8080/healthz")
		if err != nil || r.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = r.Body.Close()
		return
	}

	configFile := env("CONFIG_FILE", "/config/config.json")
	cfg, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	app := &App{
		cfg: cfg, smtp: loadSMTP(), states: make(map[string]*TargetState),
		stateFile: env("STATE_FILE", "/data/state.json"),
		client: &http.Client{
			Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
	app.loadState()

	if strings.EqualFold(os.Getenv("SEND_TEST_EMAIL_ON_START"), "true") {
		testState := TargetState{Name: "Email Delivery Test", URL: "SMTP2GO", Status: "up", Detail: "SMTP connection successful", LastChecked: time.Now()}
		testText := "The external monitoring container started successfully. HTML email alerts and SMTP delivery are working."
		if err := app.sendMail(cfg.SiteName+": "+testState.Name, testText, renderAlertHTML("TEST", cfg.SiteName, cfg.EmailDisplayName, testState, testText)); err != nil {
			log.Printf("test email failed: %v", err)
		} else {
			log.Printf("test email sent")
		}
	}

	for _, target := range cfg.Targets {
		t := target
		go app.monitor(t)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.health)
	mux.HandleFunc("/api/status", app.statusJSON)
	mux.HandleFunc("/", app.dashboard)
	server := &http.Server{Addr: env("LISTEN_ADDR", ":8080"), Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("monitoring %d targets; dashboard listening on %s", len(cfg.Targets), server.Addr)
	log.Fatal(server.ListenAndServe())
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.SiteName == "" {
		cfg.SiteName = "External Services"
	}
	if cfg.EmailDisplayName == "" {
		cfg.EmailDisplayName = "External Monitor"
	}
	if cfg.IntervalSeconds == 0 {
		cfg.IntervalSeconds = 60
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 10
	}
	if cfg.FailuresBeforeAlert == 0 {
		cfg.FailuresBeforeAlert = 3
	}
	if cfg.SuccessesBeforeRecovery == 0 {
		cfg.SuccessesBeforeRecovery = 2
	}
	if cfg.CertificateWarningDays == 0 {
		cfg.CertificateWarningDays = 14
	}
	if cfg.IntervalSeconds < 15 {
		return cfg, errors.New("interval_seconds must be at least 15")
	}
	if cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds >= cfg.IntervalSeconds {
		return cfg, errors.New("timeout_seconds must be positive and shorter than interval_seconds")
	}
	if len(cfg.Targets) == 0 {
		return cfg, errors.New("at least one target is required")
	}
	seen := map[string]bool{}
	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		if t.Name == "" || t.URL == "" {
			return cfg, fmt.Errorf("target %d requires name and url", i+1)
		}
		if seen[t.Name] {
			return cfg, fmt.Errorf("duplicate target name %q", t.Name)
		}
		seen[t.Name] = true
		u, err := url.Parse(t.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return cfg, fmt.Errorf("target %q has an invalid HTTP(S) url", t.Name)
		}
		if len(t.ExpectedStatus) == 0 {
			t.ExpectedStatus = []int{200}
		}
	}
	return cfg, nil
}

func loadSMTP() SMTPConfig {
	port := env("SMTP_PORT", "587")
	to := []string{}
	for _, address := range strings.Split(os.Getenv("SMTP_TO"), ",") {
		if v := strings.TrimSpace(address); v != "" {
			to = append(to, v)
		}
	}
	return SMTPConfig{Host: os.Getenv("SMTP_HOST"), Port: port, TLSMode: strings.ToLower(env("SMTP_TLS", "starttls")), Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), From: os.Getenv("SMTP_FROM"), To: to}
}

func (a *App) monitor(t Target) {
	a.check(t)
	ticker := time.NewTicker(time.Duration(a.cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.check(t)
	}
}

func (a *App) check(t Target) {
	started := time.Now()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, t.URL, nil)
	req.Header.Set("User-Agent", "ExternalMonitor/1.0")
	resp, err := a.client.Do(req)
	ok, detail, status, certExpiry := false, "", 0, time.Time{}
	if err != nil {
		detail = err.Error()
	} else {
		defer resp.Body.Close()
		status = resp.StatusCode
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			detail = "response read failed: " + readErr.Error()
		} else {
			ok = containsInt(t.ExpectedStatus, status)
			if !ok {
				detail = fmt.Sprintf("unexpected HTTP status %d", status)
			}
			if ok && t.Contains != "" && !bytes.Contains(body, []byte(t.Contains)) {
				ok = false
				detail = "expected response text was not found"
			}
			if ok {
				detail = fmt.Sprintf("HTTP %d", status)
			}
		}
		if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
			certExpiry = resp.TLS.PeerCertificates[0].NotAfter
		}
	}

	a.mu.Lock()
	s := a.states[t.Name]
	if s == nil {
		s = &TargetState{Name: t.Name, URL: t.URL, Status: "unknown"}
		a.states[t.Name] = s
	}
	previous := s.Status
	s.LastChecked, s.LatencyMS, s.HTTPStatus, s.Detail, s.CertExpires = time.Now(), time.Since(started).Milliseconds(), status, detail, certExpiry
	if ok {
		s.ConsecutiveFail = 0
		s.ConsecutiveOK++
		if previous == "unknown" {
			s.Status = "up"
			s.LastChanged = time.Now()
		}
		if previous == "down" && s.ConsecutiveOK >= a.cfg.SuccessesBeforeRecovery {
			s.Status = "up"
			s.LastChanged = time.Now()
			if s.DownAlertSent {
				s.StateAlertPending = "RECOVERED"
			} else {
				s.StateAlertPending = ""
			}
		}
	} else {
		s.ConsecutiveOK = 0
		s.ConsecutiveFail++
		if previous == "unknown" {
			s.Status = "pending"
		}
		if s.ConsecutiveFail >= a.cfg.FailuresBeforeAlert && previous != "down" {
			s.Status = "down"
			s.LastChanged = time.Now()
			s.DownAlertSent = false
			s.StateAlertPending = "DOWN"
		}
	}
	current := s.Status
	certKey := ""
	if !certExpiry.IsZero() && time.Until(certExpiry) <= time.Duration(a.cfg.CertificateWarningDays)*24*time.Hour {
		certKey = certExpiry.Format("2006-01-02")
	}
	shouldCertWarn := certKey != "" && s.CertWarnedFor != certKey
	if shouldCertWarn {
		s.CertAlertPending = certKey
	}
	snapshot := *s
	a.saveStateLocked()
	a.mu.Unlock()

	log.Printf("%s: %s (%s, %d ms)", t.Name, current, detail, snapshot.LatencyMS)
	if snapshot.StateAlertPending == "DOWN" {
		body := fmt.Sprintf("%s failed %d consecutive checks.\n\nReason: %s\nURL: %s\nTime: %s", t.Name, snapshot.ConsecutiveFail, detail, t.URL, snapshot.LastChecked.Format(time.RFC1123))
		if a.notify("DOWN", snapshot, body) == nil {
			a.ackStateAlert(t.Name, "DOWN")
		}
	}
	if snapshot.StateAlertPending == "RECOVERED" {
		body := fmt.Sprintf("%s recovered after %d successful checks.\n\nResult: %s\nURL: %s\nTime: %s", t.Name, snapshot.ConsecutiveOK, detail, t.URL, snapshot.LastChecked.Format(time.RFC1123))
		if a.notify("RECOVERED", snapshot, body) == nil {
			a.ackStateAlert(t.Name, "RECOVERED")
		}
	}
	if snapshot.CertAlertPending != "" && certKey == snapshot.CertAlertPending {
		body := fmt.Sprintf("The TLS certificate for %s expires on %s.\n\nURL: %s", t.Name, certExpiry.Format(time.RFC1123), t.URL)
		if a.notify("CERTIFICATE WARNING", snapshot, body) == nil {
			a.ackCertAlert(t.Name, certKey)
		}
	}
}

func (a *App) notify(kind string, s TargetState, body string) error {
	subject := fmt.Sprintf("%s: %s", a.cfg.SiteName, s.Name)
	if err := a.sendMail(subject, body, renderAlertHTML(kind, a.cfg.SiteName, a.cfg.EmailDisplayName, s, body)); err != nil {
		log.Printf("email notification failed: %v", err)
		return err
	} else {
		log.Printf("email notification sent: %s", subject)
		return nil
	}
}

func (a *App) ackStateAlert(name, kind string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.states[name]; s != nil && s.StateAlertPending == kind {
		s.StateAlertPending = ""
		s.DownAlertSent = kind == "DOWN"
		a.saveStateLocked()
	}
}

func (a *App) ackCertAlert(name, certKey string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s := a.states[name]; s != nil && s.CertAlertPending == certKey {
		s.CertAlertPending = ""
		s.CertWarnedFor = certKey
		a.saveStateLocked()
	}
}

func (a *App) sendMail(subject, textBody, htmlBody string) error {
	c := a.smtp
	if c.Host == "" || c.From == "" || len(c.To) == 0 {
		return errors.New("SMTP is not completely configured")
	}
	if strings.ContainsAny(subject, "\r\n") || strings.ContainsAny(c.From, "\r\n") {
		return errors.New("invalid email header")
	}
	for _, recipient := range c.To {
		if strings.ContainsAny(recipient, "\r\n") {
			return errors.New("invalid recipient header")
		}
	}
	headerFrom := formatEmailAddress(a.cfg.EmailDisplayName, c.From)
	message, err := buildEmailMessage(headerFrom, c.To, subject, textBody, htmlBody)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(c.Host, c.Port)
	var client *smtp.Client
	err = nil
	tlsCfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.TLSMode == "implicit" {
		conn, dialErr := tls.Dial("tcp", addr, tlsCfg)
		if dialErr != nil {
			return dialErr
		}
		client, err = smtp.NewClient(conn, c.Host)
	} else {
		conn, dialErr := net.DialTimeout("tcp", addr, 15*time.Second)
		if dialErr != nil {
			return dialErr
		}
		client, err = smtp.NewClient(conn, c.Host)
		if err == nil && c.TLSMode == "starttls" {
			err = client.StartTLS(tlsCfg)
		}
	}
	if err != nil {
		return err
	}
	defer client.Close()
	if c.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(c.From); err != nil {
		return err
	}
	for _, recipient := range c.To {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(message); err != nil {
		_ = w.Close()
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (a *App) loadState() {
	b, err := os.ReadFile(a.stateFile)
	if err != nil {
		return
	}
	var saved map[string]*TargetState
	if json.Unmarshal(b, &saved) == nil {
		a.states = saved
	}
}

func (a *App) saveStateLocked() {
	b, err := json.MarshalIndent(a.states, "", "  ")
	if err != nil {
		log.Printf("state encode failed: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.stateFile), 0700); err != nil {
		log.Printf("state directory failed: %v", err)
		return
	}
	tmp := a.stateFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		log.Printf("state write failed: %v", err)
		return
	}
	if err := os.Rename(tmp, a.stateFile); err != nil {
		log.Printf("state replace failed: %v", err)
	}
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok\n"))
}

func (a *App) statusJSON(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	states := a.stateSliceLocked()
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"site_name": a.cfg.SiteName, "targets": states, "generated_at": time.Now()})
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.mu.RLock()
	states := a.stateSliceLocked()
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboard.Execute(w, map[string]any{"SiteName": a.cfg.SiteName, "States": states, "Now": time.Now().Format(time.RFC1123)})
}

func (a *App) stateSliceLocked() []TargetState {
	result := make([]TargetState, 0, len(a.cfg.Targets))
	for _, target := range a.cfg.Targets {
		if s := a.states[target.Name]; s != nil {
			result = append(result, *s)
		} else {
			result = append(result, TargetState{Name: target.Name, URL: target.URL, Status: "unknown"})
		}
	}
	return result
}

func containsInt(values []int, wanted int) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
