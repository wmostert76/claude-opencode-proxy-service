package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SSEState struct {
	Counter  int
	LastType string
}

type OpenAIDelta struct {
	Content          string          `json:"content,omitempty"`
	ReasoningContent any             `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCallDelta `json:"tool_calls,omitempty"`
}

type ToolCallDelta struct {
	Index    *int           `json:"index,omitempty"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function *FuncDelta     `json:"function,omitempty"`
}

type FuncDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type OpenAIChunk struct {
	Choices []struct {
		Delta        OpenAIDelta `json:"delta"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		Cost             float64 `json:"cost,omitempty"`
	} `json:"usage,omitempty"`
}

func sseEvent(evt string, data any) string {
	b, _ := json.Marshal(data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", evt, string(b))
}

func contentBlockStart(index int, block any) string {
	data := map[string]any{
		"type":          "content_block_start",
		"index":         index,
		"content_block": block,
	}
	return sseEvent("content_block_start", data)
}

func contentBlockDelta(index int, delta any) string {
	data := map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": delta,
	}
	return sseEvent("content_block_delta", data)
}

func contentBlockStop(index int) string {
	data := map[string]any{
		"type":  "content_block_stop",
		"index": index,
	}
	return sseEvent("content_block_stop", data)
}

func OpenAIDeltaToAnthropicEvents(chunk OpenAIDelta, state *SSEState) string {
	// Suppress reasoning content
	if chunk.ReasoningContent != nil {
		return ""
	}

	// Tool calls
	if len(chunk.ToolCalls) > 0 {
		tc := chunk.ToolCalls[0]
		if tc.ID != "" {
			state.Counter++
			state.LastType = "tool_use"
			name := ""
			if tc.Function != nil {
				name = tc.Function.Name
			}
			args := ""
			if tc.Function != nil {
				args = tc.Function.Arguments
			}

			startBlock := contentBlockStart(state.Counter-1, map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  name,
				"input": map[string]any{},
			})

			deltaBlock := contentBlockDelta(state.Counter-1, map[string]string{
				"type":          "input_json_delta",
				"partial_json":  args,
			})

			return startBlock + deltaBlock
		}

		// Continuation of tool arguments
		if tc.Function != nil && state.Counter > 0 {
			idx := state.Counter - 1
			if idx < 0 {
				idx = 0
			}
			return contentBlockDelta(idx, map[string]string{
				"type":         "input_json_delta",
				"partial_json": tc.Function.Arguments,
			})
		}
		return ""
	}

	// Text content
	if chunk.Content != "" {
		if state.LastType != "text" {
			state.Counter++
			state.LastType = "text"

			startBlock := contentBlockStart(state.Counter-1, map[string]any{
				"type": "text",
				"text": "",
			})

			deltaBlock := contentBlockDelta(state.Counter-1, map[string]string{
				"type": "text_delta",
				"text": chunk.Content,
			})

			return startBlock + deltaBlock
		}

		// Continuation
		idx := state.Counter - 1
		if idx < 0 {
			idx = 0
		}
		return contentBlockDelta(idx, map[string]string{
			"type": "text_delta",
			"text": chunk.Content,
		})
	}

	return ""
}

func BuildSSEStream(body string, model string) (events string, finishReason string, usage *ChunkUsage) {
	lines := strings.Split(body, "\n")
	var buf strings.Builder
	var outFinish string
	var outUsage *ChunkUsage
	state := &SSEState{}

	buf.WriteString(sseEvent("message_start", map[string]any{
		"type":    "message_start",
		"message": map[string]any{
			"id":            msgID(),
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	}))
	buf.WriteString(": ping\n\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "data: ")
		if jsonStr == "[DONE]" {
			continue
		}
		var chunk OpenAIChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].FinishReason != "" {
				outFinish = chunk.Choices[0].FinishReason
			}
			evt := OpenAIDeltaToAnthropicEvents(chunk.Choices[0].Delta, state)
			if evt != "" {
				buf.WriteString(evt)
			}
		}
		if chunk.Usage != nil {
			outUsage = &ChunkUsage{
				InputTokens:     chunk.Usage.PromptTokens,
				OutputTokens:    chunk.Usage.CompletionTokens,
				TotalTokens:     chunk.Usage.TotalTokens,
				Cost:            chunk.Usage.Cost,
			}
		}
	}

	// content_block_stop for each block
	for i := 0; i < state.Counter; i++ {
		buf.WriteString(contentBlockStop(i))
	}

	// message_delta
	stopReason := "end_turn"
	switch outFinish {
	case "length":
		stopReason = "max_tokens"
	case "tool_calls":
		stopReason = "tool_use"
	}

	buf.WriteString(sseEvent("message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]int{"output_tokens": 0},
	}))

	buf.WriteString(sseEvent("message_stop", map[string]any{
		"type": "message_stop",
	}))

	return buf.String(), outFinish, outUsage
}

type ChunkUsage struct {
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"completion_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Cost            float64 `json:"cost"`
}
