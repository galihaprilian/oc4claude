# oc4claude Implementation Plan

## Phase 1: Project Setup

### 1.1 Repository & Go Module
- [ ] Initialize Go module: `go mod init github.com/galihaprilian/oc4claude`
- [ ] Create directory structure:
  ```
  cmd/oc4claude/     # CLI entry point
  internal/
    proxy/           # Proxy server logic
    transform/       # Request/response transformation
    router/          # Model routing logic
    breaker/         # Circuit breaker implementation
    tokenizer/       # Token counting
    config/          # Configuration handling
  pkg/
    anthropic/       # Anthropic API types
    openai/          # OpenAI API types
  ```
- [ ] Create basic `main.go` with CLI framework
- [ ] Add dependencies to go.mod

### 1.2 Dependencies
```
github.com/pkoukk/tiktoken-go     # Token counting
gopkg.in/yaml.v3                  # Config (optional, using JSON)
```

## Phase 2: Configuration System

### 2.1 Config Package (`internal/config/`)
- [ ] Define `Config` struct matching spec
- [ ] Implement `Load(path string)` with JSON parsing
- [ ] Implement env var interpolation (`${VAR}` and `${VAR:-default}`)
- [ ] Implement `Validate()` method
- [ ] Create default config file generator
- [ ] Write config tests

### 2.2 Default Config File
- [ ] Generate `~/.oc4claude/config.json` on first run if not exists
- [ ] Include all spec fields with sensible defaults

## Phase 3: Data Models

### 3.1 Anthropic Types (`pkg/anthropic/`)
- [ ] `MessageRequest` - POST /v1/messages request
- [ ] `MessageResponse` - Standard response
- [ ] `StreamChunk` - SSE streaming chunk types:
  - `content_block_delta`
  - `content_block_start`
  - `content_block_stop`
  - `message_delta`
  - `message_stop`
  - `ping`
- [ ] `ToolUse` - Tool use block
- [ ] `ToolResult` - Tool result block
- [ ] `ErrorResponse` - Error format

### 3.2 OpenAI Types (`pkg/openai/`)
- [ ] `ChatCompletionRequest` - OpenAI chat format
- [ ] `ChatCompletionResponse` - Standard response
- [ ] `StreamChunk` - SSE streaming chunk
- [ ] `ToolCall` - Function calling format
- [ ] `ToolMessage` - Tool role message

## Phase 4: Request/Response Transformation

### 4.1 Anthropic → OpenAI (`transform/request.go`)
- [ ] `TransformMessageRequest(anthropic.Request) (openai.Request, error)`
- [ ] Handle model name mapping
- [ ] Transform `tools` to `functions`
- [ ] Handle system prompt conversion
- [ ] Map `thinking` block to `completion_window`

### 4.2 OpenAI → Anthropic (`transform/response.go`)
- [ ] `TransformResponse(openai.Response) (anthropic.Response, error)`
- [ ] `TransformStreamChunk(openai.StreamChunk) []anthropic.StreamChunk`
- [ ] Handle `content` → `text` transformation
- [ ] Handle `function_call` → `tool_use` transformation

### 4.3 Streaming Transformation (`transform/stream.go`)
- [ ] `StreamTransformer` - io.Reader wrapper
- [ ] SSE line parsing
- [ ] Real-time chunk transformation
- [ ] Buffer management for incomplete JSON

## Phase 5: Token Counting

### 5.1 Tokenizer (`internal/tokenizer/`)
- [ ] Initialize tiktoken with `cl100k_base`
- [ ] `CountTokens(text string) int`
- [ ] `CountRequestTokens(req anthropic.Request) (prompt, completion int)`
- [ ] Cache encoding for performance

### 5.2 Context Detection
- [ ] `IsApproachingLimit(req, limit int) bool`
- [ ] Return appropriate model based on token count

## Phase 6: Model Routing & Circuit Breaker

### 6.1 Router (`internal/router/`)
- [ ] `Router` struct with model configs
- [ ] `GetModel(ctx context, req) string` - select model
- [ ] `DetectContext(req) string` - default/thinking/long_context/background
- [ ] Model URL mapping

### 6.2 Circuit Breaker (`internal/breaker/`)
- [ ] `Breaker` struct with per-model state
- [ ] States: CLOSED, OPEN, HALF_OPEN
- [ ] `RecordSuccess(model string)`
- [ ] `RecordFailure(model string)`
- [ ] `IsAvailable(model string) bool`
- [ ] Background recovery goroutine

### 6.3 Fallback Chain
- [ ] `Chain` struct wrapping breaker + model list
- [ ] `GetNext(available []string) string`
- [ ] Iterate through fallback models

## Phase 7: Proxy Server

### 7.1 HTTP Server (`internal/proxy/server.go`)
- [ ] `Server` struct with config, router, breaker
- [ ] `POST /v1/messages` handler
- [ ] `GET /v1/messages/streaming` handler
- [ ] `GET /health` handler
- [ ] `GET /status` handler
- [ ] Request logging middleware
- [ ] Error recovery middleware

### 7.2 Request Pipeline
- [ ] Parse incoming Anthropic request
- [ ] Route to appropriate model
- [ ] Transform to OpenAI format
- [ ] Forward to upstream
- [ ] Transform response (or stream)
- [ ] Return to client

### 7.3 Streaming Handler
- [ ] Upstream SSE connection
- [ ] Transform stream chunks on-the-fly
- [ ] Handle connection errors
- [ ] Graceful close

## Phase 8: CLI Interface

### 8.1 Main Command (`cmd/oc4claude/main.go`)
- [ ] Global flags: `--config`, `--verbose`
- [ ] Subcommands: `start`, `stop`, `restart`, `status`, `logs`, `install`, `uninstall`, `validate`

### 8.2 Start Command
- [ ] Load configuration
- [ ] Validate config
- [ ] Initialize server components
- [ ] Start in foreground or background based on config

### 8.3 Background Mode
- [ ] Fork process with `syscall.Setsid`
- [ ] Redirect stdio to log file
- [ ] Write PID file
- [ ] Create Unix socket for IPC

### 8.4 Control Commands
- [ ] `stop` - Send SIGTERM via PID file
- [ ] `restart` - Stop then start
- [ ] `status` - Check PID + socket connectivity
- [ ] `logs` - Tail recent log entries
- [ ] `validate` - Parse and validate config

## Phase 9: Auto-start Installation

### 9.1 Install (`cmd/oc4claude/install.go`)
- [ ] Detect Linux/WSL environment
- [ ] Create `~/.config/autostart/` if needed
- [ ] Generate `.desktop` file
- [ ] Set permissions

### 9.2 Desktop Entry
```desktop
[Desktop Entry]
Type=Application
Name=oc4claude
Exec=/path/to/oc4claude start
Hidden=false
X-GNOME-Autostart-enabled=true
```

## Phase 10: Testing & Refinement

### 10.1 Unit Tests
- [ ] Transform tests (round-trip validation)
- [ ] Config loading tests
- [ ] Circuit breaker state tests
- [ ] Token counting tests

### 10.2 Integration Tests
- [ ] Start proxy, send sample request
- [ ] Verify streaming works
- [ ] Test fallback chain (mock failures)
- [ ] Test circuit breaker behavior

### 10.3 Error Handling Polish
- [ ] Timeout handling
- [ ] Connection retry
- [ ] Graceful degradation

## Phase 11: Documentation

### 11.1 README.md
- [ ] Project overview
- [ ] Quick start guide
- [ ] Configuration reference
- [ ] Command reference
- [ ] Troubleshooting

### 11.2 Inline Documentation
- [ ] Godoc comments on public types
- [ ] README for each package

## Implementation Order

1. Project setup & config
2. Data models (Anthropic + OpenAI)
3. Request transformation
4. Response transformation (including streaming)
5. Token counting
6. Router + Circuit breaker
7. Proxy server core
8. CLI + background mode
9. Auto-start
10. Testing
11. Documentation

## Estimated Timeline

- Phase 1-3: 2-3 hours
- Phase 4-6: 4-6 hours
- Phase 7-9: 4-5 hours
- Phase 10-11: 2-3 hours

**Total: ~12-17 hours of development**
