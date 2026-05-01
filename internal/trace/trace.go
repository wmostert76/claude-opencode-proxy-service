package trace

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

type Usage struct {
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	TotalTokens  int     `json:"totalTokens"`
	Cost         float64 `json:"cost,omitempty"`
}

type Failover struct {
	From string `json:"from"`
	To   string `json:"to"`
	At   string `json:"at"`
}

type Trace struct {
	ID             string     `json:"id"`
	TS             string     `json:"ts"`
	Model          string     `json:"model"`
	FinalModel     string     `json:"finalModel"`
	Status         string     `json:"status"`
	UpstreamStatus int        `json:"upstreamStatus,omitempty"`
	LatencyMs      int64      `json:"latencyMs"`
	Stream         bool       `json:"stream"`
	Retries        int        `json:"retries"`
	Failovers      []Failover `json:"failovers,omitempty"`
	Usage          *Usage     `json:"usage,omitempty"`
	Error          string     `json:"error,omitempty"`
}

var (
	mu    sync.Mutex
	file  *os.File
	once  sync.Once

	reKey   = regexp.MustCompile(`sk-[A-Za-z0-9_-]{12,}`)
	reEmail = regexp.MustCompile(`[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}`)
)

func path() string {
	if p := os.Getenv("CLAUDE_GO_TRACE_LOG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "claude-go", "traces.jsonl")
}

func initFile() {
	once.Do(func() {
		os.MkdirAll(filepath.Dir(path()), 0o700)
		f, _ := os.OpenFile(path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		file = f
	})
}

func Redact(s string) string {
	s = reKey.ReplaceAllString(s, "sk-REDACTED")
	s = reEmail.ReplaceAllString(s, "[email-redacted]")
	return s
}

func Write(t Trace) {
	initFile()
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return
	}
	b, _ := json.Marshal(t)
	b = append(b, '\n')
	file.Write(b)
}

func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func Close() {
	if file != nil {
		file.Close()
	}
}
