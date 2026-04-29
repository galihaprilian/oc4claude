package transform

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/galihaprilian/oc4claude/pkg/anthropic"
	"github.com/galihaprilian/oc4claude/pkg/openai"
)

var (
	ErrNilChunk         = errors.New("chunk is nil")
	ErrInvalidSSE       = errors.New("invalid SSE format")
	ErrStreamTransform  = errors.New("stream transformation failed")
	ErrMissingDataField = errors.New("missing data field in SSE")
)

const (
	SSEDataPrefix = "data: "
)

type StreamTransformer struct {
	reader      *bufio.Reader
	partialLine string
}

func NewStreamTransformer(r io.Reader) *StreamTransformer {
	return &StreamTransformer{
		reader: bufio.NewReader(r),
	}
}

func (t *StreamTransformer) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	line, isPrefix, err := t.reader.ReadLine()
	if err != nil {
		if err == io.EOF && len(t.partialLine) > 0 {
			lineBytes := []byte(t.partialLine)
			t.partialLine = ""
			return copy(p, lineBytes), io.EOF
		}
		return 0, err
	}

	t.partialLine += string(line)
	if isPrefix {
		return 0, nil
	}

	if t.partialLine == "" {
		return 0, nil
	}

	currentLine := t.partialLine
	t.partialLine = ""

	if !isSSEDataLine(currentLine) {
		return 0, nil
	}

	data := extractSSEData(currentLine)
	if data == "" {
		return 0, nil
	}

	if data == "[DONE]" {
		stopChunk := anthropic.MessageStop{Type: "message_stop"}
		chunkBytes, _ := json.Marshal(stopChunk)
		result := fmt.Sprintf("data: %s\n\n", string(chunkBytes))
		return copy(p, []byte(result)), nil
	}

	openaiChunk, err := parseOpenAIChunk([]byte(data))
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrStreamTransform, err)
	}

	anthropicChunks := transformStreamChunk(openaiChunk)
	if len(anthropicChunks) == 0 {
		return 0, nil
	}

	var result strings.Builder
	for _, chunk := range anthropicChunks {
		chunkBytes, err := json.Marshal(chunk)
		if err != nil {
			continue
		}
		result.WriteString("data: ")
		result.Write(chunkBytes)
		result.WriteString("\n\n")
	}

	return copy(p, []byte(result.String())), nil
}

func isSSEDataLine(line string) bool {
	return len(line) > 6 && strings.HasPrefix(line, "data: ")
}

func extractSSEData(line string) string {
	if len(line) > 6 {
		return strings.TrimPrefix(line, "data: ")
	}
	return ""
}

func parseOpenAIChunk(data []byte) (map[string]any, error) {
	var chunk map[string]any
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSSE, err)
	}
	return chunk, nil
}

func transformStreamChunk(chunk map[string]any) []any {
	if chunk == nil {
		return nil
	}

	choicesRaw, ok := chunk["choices"].([]any)
	if !ok || len(choicesRaw) == 0 {
		return nil
	}

	choiceMap, ok := choicesRaw[0].(map[string]any)
	if !ok {
		return nil
	}

	var chunks []any
	content, hasContent := choiceMap["content"]
	finishReason, hasFinish := choiceMap["finish_reason"]

	if hasContent {
		contentStr, isString := content.(string)
		if isString && contentStr != "" {
			delta := anthropic.ContentBlockDelta{
				Type:  "content_block_delta",
				Index: 0,
				ContentBlock: anthropic.ContentBlock{
					Type: "text",
				},
				Delta: anthropic.Delta{
					Type: "text_delta",
					Text: contentStr,
				},
			}
			chunks = append(chunks, delta)
		}
	}

	if toolCallsRaw, ok := choiceMap["tool_calls"].([]any); ok {
		for i, tcRaw := range toolCallsRaw {
			if tcMap, ok := tcRaw.(map[string]any); ok {
				funcMap, _ := tcMap["function"].(map[string]any)
				name, _ := funcMap["name"].(string)
				args, _ := funcMap["arguments"].(string)
				id, _ := tcMap["id"].(string)

				delta := anthropic.ContentBlockDelta{
					Type:  "content_block_delta",
					Index: i,
					ContentBlock: anthropic.ContentBlock{
						Type:      "tool_use",
						ID:        id,
						Name:      name,
						ToolUseID: id,
					},
					Delta: anthropic.Delta{
						Type:        "input_json_delta",
						PartialJson: args,
					},
				}
				chunks = append(chunks, delta)
			}
		}
	}

	if hasFinish && finishReason != nil {
		messageStop := anthropic.MessageStop{
			Type: "message_stop",
		}
		chunks = append(chunks, messageStop)

		if usageRaw, ok := chunk["usage"].(map[string]any); ok {
			inputTokens, _ := usageRaw["prompt_tokens"].(float64)
			outputTokens, _ := usageRaw["completion_tokens"].(float64)

			messageDelta := anthropic.MessageDelta{
				Type: "message_delta",
				Delta: anthropic.Delta{
					Type: "",
				},
				Usage: anthropic.Usage{
					InputTokens:  int(inputTokens),
					OutputTokens: int(outputTokens),
				},
				StopReason: mapFinishReason(fmt.Sprintf("%v", finishReason)),
			}
			chunks = append(chunks, messageDelta)
		}
	}

	return chunks
}

type StreamingRequest struct {
	Model       string
	System      string
	Messages    []anthropic.Message
	MaxTokens   int
	Temperature float64
	Tools       []anthropic.Tool
}

func TransformStreamingRequest(req *StreamingRequest) (*openai.ChatCompletionRequest, error) {
	anthropicReq := &anthropic.MessageRequest{
		Model:       req.Model,
		System:      req.System,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       req.Tools,
		Stream:      true,
	}

	return TransformMessageRequest(anthropicReq, "")
}

func TransformSSEToWriter(w io.Writer, openaiChunk map[string]any) error {
	if openaiChunk == nil {
		return ErrNilChunk
	}

	chunks := transformStreamChunk(openaiChunk)
	for _, chunk := range chunks {
		data, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrStreamTransform, err)
		}

		line := fmt.Sprintf("data: %s\n\n", string(data))
		if _, err := w.Write([]byte(line)); err != nil {
			return err
		}
	}

	return nil
}

func ParseSSEStream(reader io.Reader) <-chan []byte {
	ch := make(chan []byte, 1)

	go func() {
		defer close(ch)
		reader2 := bufio.NewReader(reader)
		var partial string

		for {
			line, isPrefix, err := reader2.ReadLine()
			if err != nil {
				if partial != "" && len(ch) == 0 {
					ch <- []byte(partial)
				}
				return
			}

			partial += string(line)
			if isPrefix {
				continue
			}

			if partial == "" {
				continue
			}

			if !isSSEDataLine(partial) {
				partial = ""
				continue
			}

			data := extractSSEData(partial)
			partial = ""

			if data == "" {
				continue
			}

			if data == "[DONE]" {
				ch <- []byte(data)
				return
			}

			select {
			case ch <- []byte(data):
			default:
			}
		}
	}()

	return ch
}

func TransformStreamChunkBytes(openaiChunkBytes []byte) ([]byte, error) {
	var openaiChunk map[string]any
	if err := json.Unmarshal(openaiChunkBytes, &openaiChunk); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStreamTransform, err)
	}

	chunks := transformStreamChunk(openaiChunk)
	if len(chunks) == 0 {
		return nil, nil
	}

	var result strings.Builder
	for _, chunk := range chunks {
		data, err := json.Marshal(chunk)
		if err != nil {
			continue
		}
		result.WriteString("data: ")
		result.Write(data)
		result.WriteString("\n\n")
	}

	return []byte(result.String()), nil
}