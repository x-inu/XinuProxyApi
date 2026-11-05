package logging

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func TestStripANSI(t *testing.T) {
	colored := "\x1b[31mRed text\x1b[0m normal \x1b[1;32mGreen\x1b[0m"
	cleaned := StripANSI(colored)
	expected := "Red text normal Green"
	if cleaned != expected {
		t.Fatalf("StripANSI(%q) = %q, want %q", colored, cleaned, expected)
	}

	plain := "plain text without escape"
	if StripANSI(plain) != plain {
		t.Fatalf("StripANSI(%q) = %q, want plain", plain, StripANSI(plain))
	}
}

func TestConsoleLogHookAndBuffer(t *testing.T) {
	RegisterConsoleLogHook()
	ClearConsoleBuffer()

	ch, cancel := SubscribeConsoleLogs()
	defer cancel()

	log.WithField("request_id", "req-12345").Info("Testing console log buffer entry")

	select {
	case ev := <-ch:
		if ev.Type != "line" {
			t.Fatalf("expected line event, got %s", ev.Type)
		}
		if ev.Entry == nil {
			t.Fatal("expected entry to be non-nil")
		}
		if ev.Entry.RequestID != "req-12345" {
			t.Fatalf("expected request_id req-12345, got %q", ev.Entry.RequestID)
		}
		if ev.Entry.Level != "info" {
			t.Fatalf("expected level info, got %q", ev.Entry.Level)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for console log event")
	}

	buffer := GetConsoleBuffer()
	if len(buffer) == 0 {
		t.Fatal("expected console buffer to contain at least 1 entry")
	}

	ClearConsoleBuffer()
	select {
	case ev := <-ch:
		if ev.Type != "clear" {
			t.Fatalf("expected clear event, got %s", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for clear event")
	}

	if len(GetConsoleBuffer()) != 0 {
		t.Fatal("expected console buffer to be empty after clear")
	}
}

func TestSetRuntimeLogLevel(t *testing.T) {
	original := GetRuntimeLogLevel()
	defer func() {
		_, _ = SetRuntimeLogLevel(original)
	}()

	lvl, err := SetRuntimeLogLevel("debug")
	if err != nil {
		t.Fatalf("unexpected error setting level: %v", err)
	}
	if lvl != "debug" {
		t.Fatalf("expected level debug, got %s", lvl)
	}
	if GetRuntimeLogLevel() != "debug" {
		t.Fatalf("expected GetRuntimeLogLevel() to return debug, got %s", GetRuntimeLogLevel())
	}

	lvl, err = SetRuntimeLogLevel("warn")
	if err != nil {
		t.Fatalf("unexpected error setting level: %v", err)
	}
	if lvl != "warn" {
		t.Fatalf("expected level warn, got %s", lvl)
	}

	_, err = SetRuntimeLogLevel("invalid_level_xyz")
	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}
