package transform

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/galihaprilian/oc4claude/pkg/anthropic"
	"github.com/galihaprilian/oc4claude/pkg/openai"
)

var (
	ErrNilRequest       = errors.New("request is nil")
	ErrInvalidModel     = errors.New("invalid target model")
	ErrInvalidMessages  = errors.New("invalid messages")
	ErrTransformFailed  = errors.New("transformation failed")
)

var modelMappings = map[string]string{
	"claude-3-5-sonnet-20241022": "gpt-4o",
	"claude-3-5-sonnet":          "gpt-4o",
	"claude-3-opus-20240229":     "gpt-4-turbo",
	"claude-3-opus":              "gpt-4-turbo",
	"claude-3-haiku-20240307":    "gpt-4o-mini",
	"claude-3-haiku":             "gpt-4o-mini",
	"claude-3-sonnet-20240229":   "gpt-4o",
	"claude-3-sonnet":            "gpt-4o",
}

func mapModelName(anthropicModel string) string {
	if target, ok := modelMappings[anthropicModel]; ok {
		return target
	}
	return anthropicModel
}

func TransformMessageRequest(anthropicReq *anthropic.MessageRequest, targetModel string) (*openai.ChatCompletionRequest, error) {
	if anthropicReq == nil {
		return nil, ErrNilRequest
	}

	if targetModel == "" {
		targetModel = mapModelName(anthropicReq.Model)
	}

	openaiReq := &openai.ChatCompletionRequest{
		Model:       targetModel,
		Temperature: anthropicReq.Temperature,
		TopP:        anthropicReq.TopP,
		Stream:      anthropicReq.Stream,
	}

	if anthropicReq.MaxTokens > 0 {
		openaiReq.MaxTokens = anthropicReq.MaxTokens
	}

	if anthropicReq.Tools != nil && len(anthropicReq.Tools) > 0 {
		openaiReq.Tools = transformTools(anthropicReq.Tools)
	}

	if anthropicReq.ToolChoice != nil {
		openaiReq.ToolChoice = transformToolChoice(anthropicReq.ToolChoice)
	}

	messages, systemContent, err := transformMessages(anthropicReq.Messages, anthropicReq.System)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransformFailed, err)
	}

	if systemContent != "" {
		messages = prependSystemMessage(messages, systemContent)
	}

	openaiReq.Messages = messages

	return openaiReq, nil
}

func transformTools(anthropicTools []anthropic.Tool) []openai.Tool {
	openaiTools := make([]openai.Tool, len(anthropicTools))
	for i, tool := range anthropicTools {
		openaiTools[i] = openai.Tool{
			Type: "function",
			Function: &openai.Function{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  transformInputSchema(tool.InputSchema),
			},
		}
	}
	return openaiTools
}

func transformInputSchema(schema anthropic.InputSchema) map[string]any {
	if schema.Type == "object" {
		result := make(map[string]any)
		result["type"] = "object"
		if schema.Properties != nil {
			result["properties"] = schema.Properties
		}
		if schema.Required != nil {
			result["required"] = schema.Required
		}
		return result
	}
	return nil
}

func transformToolChoice(choice *anthropic.ToolChoice) interface{} {
	if choice == nil {
		return nil
	}

	switch choice.Type {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "tool":
		if choice.Name != "" {
			return map[string]string{"type": "function", "function": choice.Name}
		}
	}

	return nil
}

func transformMessages(msgs []anthropic.Message, system string) ([]openai.ChatMessage, string, error) {
	if len(msgs) == 0 && system == "" {
		return nil, "", nil
	}

	openaiMessages := make([]openai.ChatMessage, 0, len(msgs)+2)

	var processedSystem string
	if system != "" {
		processedSystem = system
	}

	for i, msg := range msgs {
		if msg.Role == "system" {
			if sysContent, ok := msg.Content.(string); ok {
				if processedSystem != "" {
					processedSystem += "\n" + sysContent
				} else {
					processedSystem = sysContent
				}
			} else if contentBlocks, ok := msg.Content.([]any); ok {
				sysText := extractTextFromContentBlocks(contentBlocks)
				if processedSystem != "" {
					processedSystem += "\n" + sysText
				} else {
					processedSystem = sysText
				}
			}
			continue
		}

		openaiMsg, err := transformMessageContent(msg, i)
		if err != nil {
			return nil, "", fmt.Errorf("message %d: %w", i, err)
		}
		openaiMessages = append(openaiMessages, *openaiMsg)
	}

	return openaiMessages, processedSystem, nil
}

func transformMessageContent(msg anthropic.Message, index int) (*openai.ChatMessage, error) {
	openaiMsg := openai.ChatMessage{
		Role: msg.Role,
	}

	if msg.Content == nil {
		openaiMsg.Content = ""
		return &openaiMsg, nil
	}

	switch content := msg.Content.(type) {
	case string:
		openaiMsg.Content = content

	case []any:
		openaiContent, err := transformContentBlocks(content)
		if err != nil {
			return nil, err
		}
		openaiMsg.Content = openaiContent

	case map[string]any:
		openaiMsg.Content = transformSingleContentBlock(content)

	default:
		openaiMsg.Content = fmt.Sprintf("%v", content)
	}

	return &openaiMsg, nil
}

func transformContentBlocks(blocks []any) (interface{}, error) {
	hasTools := false
	for _, block := range blocks {
		if blockMap, ok := block.(map[string]any); ok {
			if blockType, ok := blockMap["type"].(string); ok && blockType == "tool_use" {
				hasTools = true
				break
			}
		}
	}

	if hasTools {
		return transformContentBlocksToToolCalls(blocks)
	}

	var textParts []string
	for _, block := range blocks {
		if blockMap, ok := block.(map[string]any); ok {
			if text, ok := blockMap["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
	}
	return strings.Join(textParts, ""), nil
}

func transformContentBlocksToToolCalls(blocks []any) (interface{}, error) {
	toolCalls := make([]map[string]any, 0)
	var textParts []string

	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)

		switch blockType {
		case "text":
			if text, ok := blockMap["text"].(string); ok {
				textParts = append(textParts, text)
			}

		case "tool_use":
			toolCall := make(map[string]any)
			toolCall["id"] = blockMap["id"]
			toolCall["type"] = "function"

			funcMap := make(map[string]any)
			funcMap["name"] = blockMap["name"]

			if input, ok := blockMap["input"].(map[string]any); ok {
				inputJSON, err := json.Marshal(input)
				if err == nil {
					funcMap["arguments"] = string(inputJSON)
				}
			}

			toolCall["function"] = funcMap
			toolCalls = append(toolCalls, toolCall)

		case "thinking":
			if thinkingText, ok := blockMap["thinking"].(string); ok {
				textParts = append(textParts, thinkingText)
			}
		}
	}

	if len(toolCalls) > 0 {
		result := make(map[string]any)
		if len(textParts) > 0 {
			result["content"] = strings.Join(textParts, "")
		}
		result["tool_calls"] = toolCalls
		return result, nil
	}

	if len(textParts) > 0 {
		return strings.Join(textParts, ""), nil
	}

	return "", nil
}

func transformSingleContentBlock(content map[string]any) interface{} {
	contentType, _ := content["type"].(string)

	switch contentType {
	case "text":
		if text, ok := content["text"].(string); ok {
			return text
		}

	case "tool_use":
		toolCall := make(map[string]any)
		toolCall["id"] = content["id"]
		toolCall["type"] = "function"

		funcMap := make(map[string]any)
		funcMap["name"] = content["name"]

		if input, ok := content["input"].(map[string]any); ok {
			inputJSON, err := json.Marshal(input)
			if err == nil {
				funcMap["arguments"] = string(inputJSON)
			}
		}

		toolCall["function"] = funcMap

		return map[string]any{"tool_calls": []map[string]any{toolCall}}

	case "thinking":
		if thinking, ok := content["thinking"].(string); ok {
			return thinking
		}
	}

	return content
}

func extractTextFromContentBlocks(blocks []any) string {
	var parts []string
	for _, block := range blocks {
		if blockMap, ok := block.(map[string]any); ok {
			if text, ok := blockMap["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "")
}

func prependSystemMessage(messages []openai.ChatMessage, systemContent string) []openai.ChatMessage {
	if systemContent == "" {
		return messages
	}

	result := make([]openai.ChatMessage, 0, len(messages)+1)
	result = append(result, openai.ChatMessage{
		Role:    "system",
		Content: systemContent,
	})
	result = append(result, messages...)
	return result
}