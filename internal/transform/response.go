package transform

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/galihaprilian/oc4claude/pkg/anthropic"
	"github.com/galihaprilian/oc4claude/pkg/openai"
)

var (
	ErrNilResponse      = errors.New("response is nil")
	ErrInvalidResponse  = errors.New("invalid response format")
	ErrContentTransform = errors.New("content transformation failed")
)

func TransformResponse(openaiResp *openai.ChatCompletionResponse) (*anthropic.MessageResponse, error) {
	if openaiResp == nil {
		return nil, ErrNilResponse
	}

	if len(openaiResp.Choices) == 0 {
		return nil, ErrInvalidResponse
	}

	choice := openaiResp.Choices[0]

	anthropicResp := &anthropic.MessageResponse{
		ID:      openaiResp.ID,
		Type:    "message",
		Role:    "assistant",
		Model:   openaiResp.Model,
		Content: transformResponseContent(choice.Message),
		Usage: anthropic.Usage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	anthropicResp.StopReason = mapFinishReason(choice.FinishReason)

	return anthropicResp, nil
}

func transformResponseContent(message openai.ChatMessage) []anthropic.ContentBlock {
	var blocks []anthropic.ContentBlock

	switch content := message.Content.(type) {
	case string:
		if content != "" {
			blocks = append(blocks, anthropic.ContentBlock{
				Type: "text",
				Text: content,
			})
		}

	case []any:
		blocks = transformOpenAIMessageContent(content)

	case map[string]any:
		toolCalls, hasText := extractToolCallsAndText(content)
		if hasText {
			blocks = append(blocks, anthropic.ContentBlock{
				Type: "text",
				Text: content["content"].(string),
			})
		}
		for _, tc := range toolCalls {
			blocks = append(blocks, tc)
		}

	default:
		if contentStr := fmt.Sprintf("%v", content); contentStr != "" && contentStr != "<nil>" {
			blocks = append(blocks, anthropic.ContentBlock{
				Type: "text",
				Text: contentStr,
			})
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, anthropic.ContentBlock{
			Type: "text",
			Text: "",
		})
	}

	return blocks
}

func transformOpenAIMessageContent(content []any) []anthropic.ContentBlock {
	var blocks []anthropic.ContentBlock

	for _, item := range content {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		itemType, _ := itemMap["type"].(string)

		switch itemType {
		case "text":
			if text, ok := itemMap["text"].(string); ok {
				blocks = append(blocks, anthropic.ContentBlock{
					Type: "text",
					Text: text,
				})
			}

		case "tool_use":
			id, _ := itemMap["id"].(string)
			name, _ := itemMap["name"].(string)

			var input any
			if args, ok := itemMap["arguments"].(string); ok {
				json.Unmarshal([]byte(args), &input)
			}

			blocks = append(blocks, anthropic.ContentBlock{
				Type:      "tool_use",
				ID:        id,
				Name:      name,
				Input:     input,
				ToolUseID: id,
			})
		}
	}

	return blocks
}

func extractToolCallsAndText(content map[string]any) ([]anthropic.ContentBlock, bool) {
	var blocks []anthropic.ContentBlock
	hasText := false

	if text, ok := content["content"].(string); ok && text != "" {
		hasText = true
	}

	if toolCallsRaw, ok := content["tool_calls"].([]any); ok {
		for _, tcRaw := range toolCallsRaw {
			if tcMap, ok := tcRaw.(map[string]any); ok {
				id, _ := tcMap["id"].(string)
				name, _ := tcMap["name"].(string)

				var input any
				if args, ok := tcMap["arguments"].(string); ok {
					json.Unmarshal([]byte(args), &input)
				}

				blocks = append(blocks, anthropic.ContentBlock{
					Type:      "tool_use",
					ID:        id,
					Name:      name,
					Input:     input,
					ToolUseID: id,
				})
			}
		}
	}

	return blocks, hasText
}

func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "content_filter"
	case "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}