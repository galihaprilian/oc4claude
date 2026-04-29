package anthropic

type MessageRequest struct {
	Model           string            `json:"model"`
	System          string            `json:"system,omitempty"`
	Messages        []Message         `json:"messages"`
	MaxTokens       int               `json:"max_tokens"`
	StopSequences   []string          `json:"stop_sequences,omitempty"`
	Temperature     float64           `json:"temperature,omitempty"`
	TopP            float64           `json:"top_p,omitempty"`
	Tools           []Tool            `json:"tools,omitempty"`
	ToolChoice      *ToolChoice       `json:"tool_choice,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Stream          bool              `json:"stream,omitempty"`
	Beta            string            `json:"beta,omitempty"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ContentBlockText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ContentBlockToolUse struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
}

type ContentBlockToolResult struct {
	Type    string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content string `json:"content"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"input_schema"`
}

type InputSchema struct {
	Type       string      `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string    `json:"required,omitempty"`
}

type ToolChoice struct {
	Type    string  `json:"type"`
	Name    string  `json:"name,omitempty"`
	Disable bool    `json:"disable,omitempty"`
}

type MessageResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []ContentBlock  `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stop_reason,omitempty"`
	StopSequence string          `json:"stop_sequence,omitempty"`
	Usage        Usage           `json:"usage"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Input any   `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content string `json:"content,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type StreamChunk struct {
	Type string `json:"type"`
}

type ContentBlockDelta struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock ContentBlock `json:"content_block,omitempty"`
	Delta        Delta  `json:"delta,omitempty"`
}

type Delta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	PartialJson  string `json:"partial_json,omitempty"`
}

type ContentBlockStart struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

type ContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type MessageDelta struct {
	Type       string    `json:"type"`
	Delta      Delta     `json:"delta"`
	Usage      Usage     `json:"usage"`
	StopReason string    `json:"stop_reason,omitempty"`
}

type MessageStop struct {
	Type string `json:"type"`
}

type Ping struct {
	Type string `json:"type"`
}

type ErrorResponse struct {
	Type    string `json:"type"`
	Error   Error  `json:"error"`
}

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}