package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"targets":[{"name":"Example","url":"https://example.com"}]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IntervalSeconds != 60 || cfg.TimeoutSeconds != 10 {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if len(cfg.Targets[0].ExpectedStatus) != 1 || cfg.Targets[0].ExpectedStatus[0] != 200 {
		t.Fatal("expected status default not applied")
	}
}

func TestLoadConfigRejectsUnsafeURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"targets":[{"name":"Bad","url":"file:///etc/passwd"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected invalid URL to be rejected")
	}
}

func TestContainsInt(t *testing.T) {
	if !containsInt([]int{200, 204}, 204) || containsInt([]int{200}, 500) {
		t.Fatal("containsInt returned the wrong result")
	}
}

func TestBuildEmailMessageIsMultipart(t *testing.T) {
	message, err := buildEmailMessage("monitor@example.com", []string{"owner@example.com"}, "[TEST] Monitor", "Plain fallback", "<strong>HTML alert</strong>")
	if err != nil {
		t.Fatal(err)
	}
	got := string(message)
	for _, wanted := range []string{"Content-Type: multipart/alternative", "Content-Type: text/plain", "Content-Type: text/html", "Plain fallback", "HTML alert"} {
		if !strings.Contains(got, wanted) {
			t.Fatalf("message missing %q", wanted)
		}
	}
}

func TestRenderAlertHTMLEscapesValues(t *testing.T) {
	state := TargetState{Name: `<script>alert("x")</script>`, URL: "https://example.com/?a=1&b=2", Status: "down", Detail: "HTTP 500", LastChecked: time.Now()}
	got := renderAlertHTML("DOWN", "Example", "Example Monitor", state, "Failed")
	if strings.Contains(got, "<script>") {
		t.Fatal("unescaped target name in HTML email")
	}
	if !strings.Contains(got, "Service unavailable") || !strings.Contains(got, "#dc2626") {
		t.Fatal("down styling is missing")
	}
}

func TestFormatEmailAddress(t *testing.T) {
	got := formatEmailAddress("Example Monitor", "monitor@example.com")
	if got != `"Example Monitor" <monitor@example.com>` {
		t.Fatalf("unexpected From header: %s", got)
	}
}
