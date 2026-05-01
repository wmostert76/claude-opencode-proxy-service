package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wmostert76/claude-go/internal/trace"
)

type Server struct {
	APIKey       string
	DefaultModel string
	Port         int
	logger       *log.Logger
}

func NewServer(apiKey, defaultModel string, port int) *Server {
	logPath := os.Getenv("CLAUDE_GO_LOG")
	if logPath == "" {
		home, _ := os.UserHomeDir()
		logPath = filepath.Join(home, ".cache", "claude-go", "proxy.log")
	}
	os.MkdirAll(filepath.Dir(logPath), 0o700)
	logFile, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	var writer io.Writer = os.Stderr
	if logFile != nil {
		writer = io.MultiWriter(os.Stderr, logFile)
	}
	return &Server{
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		Port:         port,
		logger:       log.New(writer, "[claude-go] ", log.LstdFlags),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/", s.handleRequest)

	addr := fmt.Sprintf("127.0.0.1:%d", s.Port)
	s.logger.Printf("[claude-go] listening on http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"target": Target,
		"model":  s.DefaultModel,
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	models, err := FetchModels(s.APIKey)
	if err != nil || len(models) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []any{},
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   models,
	})
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}

	if r.Method == "GET" {
		w.WriteHeader(405)
		w.Write([]byte("Method Not Allowed"))
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(405)
		w.Write([]byte("Method Not Allowed"))
		return
	}

	// Token counting
	if strings.Contains(r.URL.Path, "count_tokens") {
		s.handleTokenCount(w, r)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	var areq AnthropicRequest
	if err := json.Unmarshal(body, &areq); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	oreq := AnthropicToOpenAI(areq, s.DefaultModel)
	tr := &trace.Trace{
		ID:     r.Header.Get("x-claude-opencode-trace-id"),
		TS:     time.Now().Format(time.RFC3339),
		Model:  oreq.Model,
		Status: "started",
		Stream: oreq.Stream,
	}
	if tr.ID == "" {
		tr.ID = trace.NewID()
	}

	failoverEnabled := r.Header.Get("x-claude-opencode-no-failover") != "1"
	w.Header().Set("x-claude-opencode-trace-id", tr.ID)

	started := time.Now()

	resp, err := ForwardRequest(oreq, tr, s.APIKey, failoverEnabled)
	if err != nil {
		tr.Status = "error"
		tr.LatencyMs = time.Since(started).Milliseconds()
		tr.Error = trace.Redact(err.Error())
		trace.Write(*tr)
		s.logger.Printf("[claude-go] %s error: %s", tr.ID, err.Error())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "api_error",
				"message": err.Error(),
			},
		})
		return
	}
	defer resp.Body.Close()

	tr.UpstreamStatus = resp.StatusCode

	if resp.StatusCode >= 400 {
		errText, _ := io.ReadAll(resp.Body)
		tr.Status = "error"
		tr.LatencyMs = time.Since(started).Milliseconds()
		tr.Error = trace.Redact(string(errText))
		if len(tr.Error) > 500 {
			tr.Error = tr.Error[:500]
		}
		trace.Write(*tr)
		s.logger.Printf("[claude-go] %s upstream error %d: %s", tr.ID, resp.StatusCode, tr.Error)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "api_error",
				"message": fmt.Sprintf("Upstream error: %d", resp.StatusCode),
			},
		})
		return
	}

	if oreq.Stream {
		// Streaming
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(200)

		bodyText, _ := io.ReadAll(resp.Body)
		events, _, chunkUsage := BuildSSEStream(string(bodyText), oreq.Model)
		w.Write([]byte(events))

		if chunkUsage != nil {
			tr.Usage = &trace.Usage{
				InputTokens:  chunkUsage.InputTokens,
				OutputTokens: chunkUsage.OutputTokens,
				TotalTokens:  chunkUsage.TotalTokens,
				Cost:         chunkUsage.Cost,
			}
		}
		tr.Status = "ok"
		tr.LatencyMs = time.Since(started).Milliseconds()
		trace.Write(*tr)
	} else {
		// Non-streaming
		var oresp OpenAIResponse
		json.NewDecoder(resp.Body).Decode(&oresp)

		if oresp.Usage.TotalTokens > 0 || oresp.Usage.PromptTokens > 0 {
			tr.Usage = &trace.Usage{
				InputTokens:  oresp.Usage.PromptTokens,
				OutputTokens: oresp.Usage.CompletionTokens,
				TotalTokens:  oresp.Usage.TotalTokens,
				Cost:         oresp.Cost,
			}
		}

		aresp := OpenAIToAnthropic(oresp)
		tr.Status = "ok"
		tr.LatencyMs = time.Since(started).Milliseconds()
		trace.Write(*tr)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(aresp)
	}
}

func (s *Server) handleTokenCount(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	json.Unmarshal(body, &req)

	var text string
	if msgs, ok := req["messages"].([]any); ok {
		for _, m := range msgs {
			msg, _ := m.(map[string]any)
			if c, ok := msg["content"].(string); ok {
				text += c + " "
			}
		}
	}
	if sys, ok := req["system"]; ok {
		if s, ok := sys.(string); ok {
			text += s + " "
		} else {
			b, _ := json.Marshal(sys)
			text += string(b) + " "
		}
	}

	words := strings.Fields(text)
	tokens := int(float64(len(words)) * 1.3)
	if tokens < 0 {
		tokens = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"input_tokens": tokens,
	})
}

func Port() int {
	p := os.Getenv("PROXY_PORT")
	if p == "" {
		return 8082
	}
	n, err := strconv.Atoi(p)
	if err != nil || n <= 0 {
		return 8082
	}
	return n
}

func APIKey() string {
	return os.Getenv("ANTHROPIC_API_KEY")
}

func DefaultModel() string {
	m := os.Getenv("OPENCODE_GO_MODEL")
	if m == "" {
		return "deepseek-v4-pro"
	}
	return m
}

// ProxyInfo prints startup banner
func ProxyInfo(version string) {
	fmt.Printf("Claude Go\n")
	fmt.Printf("─────────────────────────────\n")
	fmt.Printf("  Release    v%s\n", version)
	fmt.Printf("  Proxy      http://127.0.0.1:%d\n", Port())
	fmt.Printf("  Provider   OpenCode Go\n")
	fmt.Printf("  Model      auto (via /v1/models discovery)\n")
	fmt.Printf("  Tools      all Claude Code native tools\n")
	fmt.Printf("─────────────────────────────\n\n")
}
