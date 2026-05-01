package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wmostert76/claude-go/internal/trace"
)

const (
	Target    = "https://opencode.ai/zen/go/v1/chat/completions"
	ModelsURL = "https://opencode.ai/zen/go/v1/models"
)

var (
	retryStatuses = map[int]bool{
		408: true, 409: true, 425: true,
		429: true, 500: true, 502: true, 503: true, 504: true,
	}
	fallbackModels []string
)

func init() {
	fm := os.Getenv("CLAUDE_GO_FALLBACK_MODELS")
	if fm == "" {
		fm = "glm-5.1,kimi-k2.6,minimax-m2.7,qwen3.6-plus"
	}
	for _, m := range strings.Split(fm, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			fallbackModels = append(fallbackModels, m)
		}
	}
}

func RetryAttempts() int {
	n, err := strconv.Atoi(os.Getenv("CLAUDE_GO_RETRY_ATTEMPTS"))
	if err != nil || n < 0 {
		return 2
	}
	return n
}

func RetryBaseMs() int {
	n, err := strconv.Atoi(os.Getenv("CLAUDE_GO_RETRY_BASE_MS"))
	if err != nil || n <= 0 {
		return 350
	}
	return n
}

func candidateModels(primary string, failoverEnabled bool) []string {
	if !failoverEnabled {
		return []string{primary}
	}
	seen := map[string]bool{primary: true}
	models := []string{primary}
	for _, m := range fallbackModels {
		if !seen[m] {
			seen[m] = true
			models = append(models, m)
		}
	}
	return models
}

func ForwardRequest(req OpenAIRequest, tr *trace.Trace, apiKey string, failoverEnabled bool) (*http.Response, error) {
	models := candidateModels(req.Model, failoverEnabled)
	attempts := RetryAttempts()
	baseDelay := time.Duration(RetryBaseMs()) * time.Millisecond

	var lastResp *http.Response
	var lastErr error

	for _, model := range models {
		reqForModel := req
		reqForModel.Model = model

		if strings.HasPrefix(model, "deepseek-") {
			reqForModel.Thinking = map[string]string{"type": "disabled"}
		} else {
			reqForModel.Thinking = nil
		}

		if model != req.Model {
			tr.Failovers = append(tr.Failovers, trace.Failover{
				From: req.Model,
				To:   model,
				At:   time.Now().Format(time.RFC3339),
			})
		}
		tr.FinalModel = model

		for i := 0; i <= attempts; i++ {
			if i > 0 {
				tr.Retries++
				time.Sleep(baseDelay * time.Duration(i))
			}

			body, err := json.Marshal(reqForModel)
			if err != nil {
				lastErr = err
				break
			}

			httpReq, err := http.NewRequest("POST", Target, bytes.NewReader(body))
			if err != nil {
				lastErr = err
				break
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)

			tr.UpstreamStatus = 0
			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				tr.Error = trace.Redact(err.Error())
				lastErr = err
				continue
			}

			lastResp = resp
			tr.UpstreamStatus = resp.StatusCode

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp, nil
			}

			if !retryStatuses[resp.StatusCode] {
				return resp, nil
			}

			resp.Body.Close()
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("upstream request failed")
}
