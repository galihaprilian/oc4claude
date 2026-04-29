package openai

type ChatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []ChatMessage     `json:"messages"`
	Tools       []Tool            `json:"tools,omitempty"`
	ToolChoice  interface{}       `json:"tool_choice,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	TopP        float64           `json:"top_p,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	PresencePenalty float64       `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64      `json:"frequency_penalty,omitempty"`
	LogitBias   map[string]int    `json:"logit_bias,omitempty"`
	User        string            `json:"user,omitempty"`
	N           int               `json:"n,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Seed        *int              `json:"seed,omitempty"`
	ServiceTier string            `json:"service_tier,omitempty"`
	Stop        interface{}       `json:"stop,omitempty"`
	Store       bool              `json:"store,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ChatMessage struct {
	Role    string        `json:"role"`
	Content interface{}   `json:"content"`
	Name    string        `json:"name,omitempty"`
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function *Function    `json:"function,omitempty"`
}

type Function struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Parameters  map[string]any    `json:"parameters,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	ID                string     `json:"id"`
	Object            string     `json:"object"`
	Created           int64      `json:"created"`
	Model             string     `json:"model"`
	Choices           []Choice   `json:"choices"`
	Usage             *Usage     `json:"usage,omitempty"`
	XShopcartTokenID  string     `json:"x_shopcart_token_id,omitempty"`
}

type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type ToolMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}