package tokenizer

import (
	"errors"
	"strings"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
	"github.com/galihaprilian/oc4claude/pkg/anthropic"
)

var (
	encoder     *tiktoken.Tiktoken
	encoderInit sync.Once
	initErr     error
)

func initEncoder() {
	encoder, initErr = tiktoken.EncodingForModel("cl100k_base")
	if initErr != nil {
		encoder, initErr = tiktoken.GetEncoding("cl100k_base")
	}
}

func getEncoder() (*tiktoken.Tiktoken, error) {
	encoderInit.Do(initEncoder)
	if initErr != nil {
		return nil, initErr
	}
	return encoder, nil
}

func CountTokens(text string) (int, error) {
	enc, err := getEncoder()
	if err != nil {
		return 0, err
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens), nil
}

func extractContentText(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var sb strings.Builder
		for _, item := range c {
			if block, ok := item.(map[string]any); ok {
				if text, ok := block["text"].(string); ok {
					sb.WriteString(text)
				}
			}
		}
		return sb.String()
	}
	return ""
}

func CountRequestTokens(req *anthropic.MessageRequest) (promptTokens, completionTokens int, err error) {
	if req == nil {
		return 0, 0, errors.New("request is nil")
	}

	enc, err := getEncoder()
	if err != nil {
		return 0, 0, err
	}

	if req.System != "" {
		tokens := enc.Encode(req.System, nil, nil)
		promptTokens += len(tokens)
	}

	for _, msg := range req.Messages {
		content := extractContentText(msg.Content)
		tokens := enc.Encode(msg.Role+":"+content, nil, nil)
		promptTokens += len(tokens)
	}

	assistantTokens := enc.Encode("assistant", nil, nil)
	promptTokens += len(assistantTokens) + 3

	if req.MaxTokens > 0 {
		completionTokens = req.MaxTokens
	}

	return promptTokens, completionTokens, nil
}

func ApproachingTokenLimit(promptTokens, completionTokens, limit int) bool {
	if limit <= 0 {
		return false
	}
	total := promptTokens + completionTokens
	return total >= limit
}

func NeedsLongContext(promptTokens, completionTokens, limit int) bool {
	if limit <= 0 {
		return false
	}
	threshold := int(float64(limit) * 0.9)
	total := promptTokens + completionTokens
	return total >= threshold
}