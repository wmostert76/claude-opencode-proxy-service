package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
)

var goModelPrefixes = []string{"deepseek-", "glm-", "kimi-", "minimax-", "qwen", "mimo-"}

func IsGoModel(model string) bool {
	for _, p := range goModelPrefixes {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return false
}

func NormalizeModel(model, defaultModel string) string {
	if model == "" {
		return defaultModel
	}
	name := strings.TrimPrefix(model, "opencode-go/")
	name = removePrefix(name, "claude-")
	name = removePrefix(name, "anthropic-")
	if IsGoModel(name) {
		return name
	}
	return defaultModel
}

func removePrefix(s, prefix string) string {
	lower := strings.ToLower(s)
	p := strings.ToLower(prefix)
	if strings.HasPrefix(lower, p) {
		return s[len(prefix):]
	}
	return s
}

type AnthropicContent struct {
	Type       string          `json:"type"`
	Text       string          `json:"text,omitempty"`
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	ToolUseID  string          `json:"tool_use_id,omitempty"`
	Content    any             `json:"content,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type AnthropicToolChoice struct {
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

type AnthropicRequest struct {
	Model         string               `json:"model"`
	MaxTokens     int                  `json:"max_tokens"`
	Messages      []AnthropicMessage   `json:"messages"`
	System        any                  `json:"system,omitempty"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    any                  `json:"tool_choice,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	TopK          *int                 `json:"top_k,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
}

type OpenAIToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function OpenAIFunc   `json:"function"`
}

type OpenAIFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAITool struct {
	Type     string       `json:"type"`
	Function OpenAIToolFn `json:"function"`
}

type OpenAIToolFn struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type OpenAIMessage struct {
	Role      string          `json:"role"`
	Content   any             `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []OpenAIMessage `json:"messages"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	TopK        int             `json:"top_k,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Thinking    any             `json:"thinking,omitempty"`
}

type contentExtract struct {
	text      string
	toolCalls []toolCallData
	images    []imageData
}

type imageData struct {
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type toolCallData struct {
	ID   string
	Name string
	Args json.RawMessage
}

func resolveContent(v any) contentExtract {
	switch c := v.(type) {
	case string:
		return contentExtract{text: c}
	case []any:
		return resolveArrayContent(c)
	default:
		return contentExtract{}
	}
}

func resolveArrayContent(arr []any) contentExtract {
	var texts []string
	var tcs []toolCallData
	var imgs []imageData
	for _, block := range arr {
		b, ok := block.(map[string]any)
		if !ok {
			continue
		}
		switch b["type"] {
		case "text":
			if t, ok := b["text"].(string); ok {
				texts = append(texts, t)
			}
		case "tool_use":
			tc := toolCallData{}
			if id, ok := b["id"].(string); ok {
				tc.ID = id
			}
			if name, ok := b["name"].(string); ok {
				tc.Name = name
			}
			if input, ok := b["input"]; ok {
				tc.Args, _ = json.Marshal(input)
			}
			tcs = append(tcs, tc)
		case "tool_result":
			// handled separately
		case "image":
			if src, ok := b["source"].(map[string]any); ok {
				mediaType, _ := src["media_type"].(string)
				data, _ := src["data"].(string)
				if data != "" {
					imgs = append(imgs, imageData{
						MediaType: mediaType,
						Data:      data,
					})
				}
			}
		}
	}
	return contentExtract{
		text:      strings.Join(texts, "\n"),
		toolCalls: tcs,
		images:    imgs,
	}
}

func extractToolResults(v any) []OpenAIMessage {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var msgs []OpenAIMessage
	for _, block := range arr {
		b, ok := block.(map[string]any)
		if !ok || b["type"] != "tool_result" {
			continue
		}
		toolUseID, _ := b["tool_use_id"].(string)
		content := ""
		switch c := b["content"].(type) {
		case string:
			content = c
		case []any:
			var parts []string
			for _, p := range c {
				if pm, ok := p.(map[string]any); ok {
					if t, ok := pm["text"].(string); ok {
						parts = append(parts, t)
					}
				}
			}
			content = strings.Join(parts, "\n")
		}
		msgs = append(msgs, OpenAIMessage{
			Role:      "tool",
			ToolCallID: toolUseID,
			Content:   content,
		})
	}
	return msgs
}

func TranslateMessages(anthropicMsgs []AnthropicMessage) []OpenAIMessage {
	var openaiMsgs []OpenAIMessage
	for _, msg := range anthropicMsgs {
		toolMsgs := extractToolResults(msg.Content)
		openaiMsgs = append(openaiMsgs, toolMsgs...)

		ce := resolveContent(msg.Content)

		// Build content — if images present, use array format
		var content any
		if len(ce.images) > 0 && msg.Role == "user" {
			var parts []map[string]any
			if ce.text != "" {
				parts = append(parts, map[string]any{"type": "text", "text": ce.text})
			}
			for _, img := range ce.images {
				mediaType := img.MediaType
				if mediaType == "" {
					mediaType = "image/png"
				}
				parts = append(parts, map[string]any{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "data:" + mediaType + ";base64," + img.Data,
					},
				})
			}
			if len(parts) == 1 && parts[0]["type"] == "text" {
				content = parts[0]["text"]
			} else {
				content = parts
			}
		} else if ce.text != "" {
			content = ce.text
		}

		if len(ce.toolCalls) > 0 && msg.Role == "assistant" {
			var tcs []OpenAIToolCall
			for _, tc := range ce.toolCalls {
				args := string(tc.Args)
				if args == "" {
					args = "{}"
				}
				tcs = append(tcs, OpenAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: OpenAIFunc{
						Name:      tc.Name,
						Arguments: args,
					},
				})
			}
			c := any(nil)
			if content != nil {
				c = content
			}
			openaiMsgs = append(openaiMsgs, OpenAIMessage{
				Role:      "assistant",
				Content:   c,
				ToolCalls: tcs,
			})
		} else {
			if content == nil {
				content = ""
			}
			openaiMsgs = append(openaiMsgs, OpenAIMessage{
				Role:    msg.Role,
				Content: content,
			})
		}
	}
	return openaiMsgs
}

func TranslateTools(anthropicTools []AnthropicTool) []OpenAITool {
	if len(anthropicTools) == 0 {
		return nil
	}
	tools := make([]OpenAITool, len(anthropicTools))
	for i, t := range anthropicTools {
		params := t.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		tools[i] = OpenAITool{
			Type: "function",
			Function: OpenAIToolFn{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}
	return tools
}

func TranslateToolChoice(tc any) any {
	switch v := tc.(type) {
	case string:
		switch v {
		case "any", "required":
			return "required"
		case "auto":
			return "auto"
		default:
			return "auto"
		}
	case map[string]any:
		t, _ := v["type"].(string)
		switch t {
		case "any":
			return "required"
		case "tool":
			name, _ := v["name"].(string)
			return map[string]any{
				"type": "function",
				"function": map[string]any{"name": name},
			}
		default:
			return "auto"
		}
	default:
		return "auto"
	}
}

func HasImageContent(areq AnthropicRequest) bool {
	for _, msg := range areq.Messages {
		ce := resolveContent(msg.Content)
		if len(ce.images) > 0 {
			return true
		}
	}
	return false
}

var VisionModel = "kimi-k2.6"

var visionModels = map[string]bool{
	"kimi-k2.6": true,
	"kimi-k2.5": true,
	"glm-5.1":   true,
	"glm-5":     true,
}

func NeedsVisionModel(areq AnthropicRequest, currentModel string) (string, bool) {
	if !HasImageContent(areq) {
		return currentModel, false
	}
	if visionModels[currentModel] {
		return currentModel, false
	}
	return VisionModel, true
}

func AnthropicToOpenAI(areq AnthropicRequest, defaultModel string) OpenAIRequest {
	model := NormalizeModel(areq.Model, defaultModel)
	maxTokens := areq.MaxTokens
	if maxTokens < 500 {
		maxTokens = 500
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := OpenAIRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  TranslateMessages(areq.Messages),
		Tools:     TranslateTools(areq.Tools),
		Stream:    areq.Stream,
		Stop:      areq.StopSequences,
	}

	if areq.ToolChoice != nil {
		req.ToolChoice = TranslateToolChoice(areq.ToolChoice)
	} else {
		req.ToolChoice = "auto"
	}

	if areq.Temperature != nil {
		req.Temperature = *areq.Temperature
	}
	if areq.TopP != nil {
		req.TopP = *areq.TopP
	}
	if areq.TopK != nil {
		req.TopK = *areq.TopK
	}

	// System prompt
	sysText := resolveSystemPrompt(areq.System)
	if sysText != "" {
		sysMsg := OpenAIMessage{Role: "system", Content: sysText}
		req.Messages = append([]OpenAIMessage{sysMsg}, req.Messages...)
	}

	// Disable DeepSeek thinking
	if strings.HasPrefix(model, "deepseek-") {
		req.Thinking = map[string]string{"type": "disabled"}
	}

	return req
}

func resolveSystemPrompt(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []any:
		var parts []string
		for _, b := range s {
			if str, ok := b.(string); ok {
				parts = append(parts, str)
			} else if m, ok := b.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				} else if c, ok := m["content"].(string); ok {
					parts = append(parts, c)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

// ── OpenAI → Anthropic ──────────────────────────────────────────────

type AnthropicResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Role         string            `json:"role"`
	Model        string            `json:"model"`
	Content      []AnthropicContent `json:"content"`
	StopReason   string            `json:"stop_reason"`
	StopSequence any               `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type OpenAIResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role             string          `json:"role"`
			Content          any             `json:"content"`
			ReasoningContent any             `json:"reasoning_content"`
			ToolCalls        []OpenAIToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Cost float64 `json:"cost,omitempty"`
}

func msgID() string {
	return fmt.Sprintf("msg_%s", NewShortID())
}

func OpenAIToAnthropic(resp OpenAIResponse) AnthropicResponse {
	aresp := AnthropicResponse{
		ID:    resp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: resp.Model,
	}
	if aresp.ID == "" {
		aresp.ID = msgID()
	}

	if len(resp.Choices) > 0 {
		ch := resp.Choices[0]
		var blocks []AnthropicContent

		// Tool calls first
		for _, tc := range ch.Message.ToolCalls {
			var args any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			if args == nil {
				args = map[string]any{}
			}
			argsJSON, _ := json.Marshal(args)
			blocks = append(blocks, AnthropicContent{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(argsJSON),
			})
		}

		// Text
		text := ""
		if s, ok := ch.Message.Content.(string); ok {
			text = s
		}
		if text == "" {
			if s, ok := ch.Message.ReasoningContent.(string); ok {
				text = s
			}
		}
		if text != "" || len(blocks) == 0 {
			if text == "" {
				text = ""
			}
			blocks = append(blocks, AnthropicContent{
				Type: "text",
				Text: text,
			})
		}

		aresp.Content = blocks

		// Stop reason
		finish := ch.FinishReason
		if len(ch.Message.ToolCalls) > 0 {
			aresp.StopReason = "tool_use"
		} else {
			switch finish {
			case "length":
				aresp.StopReason = "max_tokens"
			case "tool_calls":
				aresp.StopReason = "tool_use"
			case "stop":
				aresp.StopReason = "end_turn"
			default:
				aresp.StopReason = "end_turn"
			}
		}
	}

	aresp.StopSequence = nil
	aresp.Usage.InputTokens = resp.Usage.PromptTokens
	aresp.Usage.OutputTokens = resp.Usage.CompletionTokens
	return aresp
}

func NewShortID() string {
	u := NewID()
	return u[:8]
}

func NewID() string {
	// fallback random ID generation
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	src := make([]byte, 32)
	for i := range src {
		src[i] = chars[i%len(chars)]
	}
	return string(src)
}
