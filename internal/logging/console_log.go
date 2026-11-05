package logging

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// DefaultConsoleMaxLines is the maximum number of log lines kept in memory.
	DefaultConsoleMaxLines = 1000
	defaultConsoleChanCap  = 256
)

var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes terminal ANSI escape sequences from rendered log lines.
func StripANSI(line string) string {
	if !strings.Contains(line, "\x1b[") {
		return line
	}
	return ansiSGR.ReplaceAllString(line, "")
}

// ConsoleEntry represents one structured console log line in memory.
type ConsoleEntry struct {
	Time      time.Time `json:"time"`
	Level     string    `json:"level"`
	Line      string    `json:"line"`
	RequestID string    `json:"request_id,omitempty"`
	Caller    string    `json:"caller,omitempty"`
	Message   string    `json:"message"`
}

// ConsoleEvent is the event payload streamed to subscribers via SSE.
type ConsoleEvent struct {
	Type    string         `json:"type"` // "init", "line", "clear"
	Entry   *ConsoleEntry  `json:"entry,omitempty"`
	Entries []ConsoleEntry `json:"entries,omitempty"`
}

type consoleBufferManager struct {
	mu       sync.Mutex
	maxLines int
	buffer   []ConsoleEntry
	subs     map[int]chan ConsoleEvent
	nextID   int
}

var (
	consoleManager = &consoleBufferManager{
		maxLines: DefaultConsoleMaxLines,
		subs:     make(map[int]chan ConsoleEvent),
	}
	registerConsoleHookOnce sync.Once
)

// ConsoleLogHook intercepts logrus entries, saves them to an in-memory ring buffer,
// and broadcasts them to active subscribers.
type ConsoleLogHook struct {
	formatter *LogFormatter
}

// Levels returns all supported logrus levels.
func (h *ConsoleLogHook) Levels() []log.Level {
	return log.AllLevels
}

// FormatConsoleMessage formats the entry message and structured fields without the redundant
// [timestamp] [req_id] [level] [caller] prefix since those are separate fields in ConsoleEntry and UI columns.
func FormatConsoleMessage(entry *log.Entry) string {
	message := strings.TrimRight(entry.Message, "\r\n")
	var fieldsStr string
	if len(entry.Data) > 0 {
		var fields []string
		for _, k := range logFieldOrder {
			if k == "request_id" {
				continue
			}
			if v, ok := entry.Data[k]; ok {
				fields = append(fields, fmt.Sprintf("%s=%s", k, formatLogFieldValue(k, v)))
			}
		}
		if pluginID, ok := entry.Data["plugin_id"]; ok && strings.TrimSpace(fmt.Sprint(pluginID)) != "" {
			for _, k := range pluginPathFieldOrder {
				if v, ok := entry.Data[k]; ok {
					fields = append(fields, fmt.Sprintf("%s=%v", k, v))
				}
			}
		}
		if len(fields) > 0 {
			fieldsStr = " " + strings.Join(fields, " ")
		}
	}
	return StripANSI(message + fieldsStr)
}

// Fire processes a log entry and records it in the ring buffer.
func (h *ConsoleLogHook) Fire(entry *log.Entry) error {
	cleanLine := FormatConsoleMessage(entry)

	levelStr := entry.Level.String()
	if levelStr == "warning" {
		levelStr = "warn"
	}

	reqID := ""
	if id, ok := entry.Data["request_id"].(string); ok && id != "" {
		reqID = id
	}

	callerStr := ""
	if entry.Caller != nil {
		callerStr = fmt.Sprintf("%s:%d", filepath.Base(entry.Caller.File), entry.Caller.Line)
	}

	cEntry := ConsoleEntry{
		Time:      entry.Time.UTC(),
		Level:     levelStr,
		Line:      cleanLine,
		RequestID: reqID,
		Caller:    callerStr,
		Message:   entry.Message,
	}

	consoleManager.record(cEntry)
	return nil
}

func (m *consoleBufferManager) record(entry ConsoleEntry) {
	m.mu.Lock()
	m.buffer = append(m.buffer, entry)
	if len(m.buffer) > m.maxLines {
		m.buffer = m.buffer[len(m.buffer)-m.maxLines:]
	}

	subCount := len(m.subs)
	if subCount == 0 {
		m.mu.Unlock()
		return
	}

	targets := make([]chan ConsoleEvent, 0, subCount)
	for _, ch := range m.subs {
		targets = append(targets, ch)
	}
	m.mu.Unlock()

	ev := ConsoleEvent{
		Type:  "line",
		Entry: &entry,
	}

	for _, ch := range targets {
		select {
		case ch <- ev:
		default:
			// Non-blocking: drop for slow subscriber to protect proxy throughput
		}
	}
}

// GetConsoleBuffer returns a copy of the current buffered log entries.
func GetConsoleBuffer() []ConsoleEntry {
	consoleManager.mu.Lock()
	defer consoleManager.mu.Unlock()

	out := make([]ConsoleEntry, len(consoleManager.buffer))
	copy(out, consoleManager.buffer)
	return out
}

// ClearConsoleBuffer clears all buffered entries and broadcasts a clear event.
func ClearConsoleBuffer() {
	consoleManager.mu.Lock()
	consoleManager.buffer = consoleManager.buffer[:0]

	targets := make([]chan ConsoleEvent, 0, len(consoleManager.subs))
	for _, ch := range consoleManager.subs {
		targets = append(targets, ch)
	}
	consoleManager.mu.Unlock()

	ev := ConsoleEvent{Type: "clear"}
	for _, ch := range targets {
		select {
		case ch <- ev:
		default:
		}
	}
}

// SubscribeConsoleLogs registers a live log stream channel.
// The caller must invoke the returned cancel function upon disconnect.
func SubscribeConsoleLogs() (<-chan ConsoleEvent, func()) {
	consoleManager.mu.Lock()
	defer consoleManager.mu.Unlock()

	id := consoleManager.nextID
	consoleManager.nextID++

	ch := make(chan ConsoleEvent, defaultConsoleChanCap)
	consoleManager.subs[id] = ch

	cancel := func() {
		consoleManager.mu.Lock()
		defer consoleManager.mu.Unlock()
		delete(consoleManager.subs, id)
	}

	return ch, cancel
}

// GetRuntimeLogLevel returns the current server log level as a string.
func GetRuntimeLogLevel() string {
	lvl := log.GetLevel().String()
	if lvl == "warning" {
		return "warn"
	}
	return lvl
}

// SetRuntimeLogLevel dynamically changes the log level at runtime without restarting the server.
func SetRuntimeLogLevel(levelStr string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(levelStr))
	if norm == "warn" {
		norm = "warning"
	}

	parsed, err := log.ParseLevel(norm)
	if err != nil {
		return "", fmt.Errorf("invalid log level %q: valid levels are debug, info, warn, error", levelStr)
	}

	log.SetLevel(parsed)
	current := GetRuntimeLogLevel()
	log.Infof("server log level changed to %s (runtime update)", current)
	return current, nil
}

// RegisterConsoleLogHook registers the console log buffer hook into logrus.
func RegisterConsoleLogHook() {
	registerConsoleHookOnce.Do(func() {
		log.AddHook(&ConsoleLogHook{
			formatter: &LogFormatter{},
		})
	})
}
