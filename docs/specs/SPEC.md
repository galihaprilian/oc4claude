# oc4claude Specification

## 1. Project Overview

**Project Name:** oc4claude
**Type:** CLI Proxy / Middleware
**Core Functionality:** A transparent proxy that intercepts Anthropic API requests from Claude Code, transforms them to OpenAI format, forwards to OpenCode Go's endpoint, and transforms responses back to Anthropic format.
**Target Users:** Developers who want to use Claude Code with OpenCode Go subscription for cost-effective AI assistance.

## 2. Architecture

```
┌─────────────┐    Anthropic API    ┌──────────────┐   OpenAI API   ┌──────────────┐
│ Claude Code │ ◄──────────────────► │  oc4claude   │ ◄────────────► │ OpenCode Go  │
└─────────────┘   (localhost proxy)  └──────────────┘               └──────────────┘
```

- **Listen Address:** Default `127.0.0.1:8080` (configurable)
- **Upstream URL:** OpenCode Go endpoint (configurable)
- **Protocol:** HTTP/S with SSE streaming support

## 3. Configuration

### 3.1 Config File: `~/.oc4claude/config.json`

```json
{
  "listen": "127.0.0.1:8080",
  "upstream_url": "https://api.opencode.ai/v1",
  "api_key": "${OPENCODE_API_KEY}",
  "default_model": "anthropic/claude-3.5-sonnet",
  "models": {
    "default": "anthropic/claude-3.5-sonnet",
    "thinking": "anthropic/claude-3.5-haiku",
    "long_context": "anthropic/claude-3.5-sonnet-20241022",
    "background": "anthropic/claude-3.5-haiku"
  },
  "fallback_chain": [
    "anthropic/claude-3.5-sonnet",
    "anthropic/claude-3.5-haiku",
    "anthropic/claude-opus"
  ],
  "circuit_breaker": {
    "enabled": true,
    "failure_threshold": 3,
    "recovery_timeout_seconds": 60
  },
  "token_limit": 180000,
  "log_level": "info",
  "background": false,
  "auto_start": false
}
```

### 3.2 Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENCODE_API_KEY` | OpenCode Go API key |
| `OC4CLAUDE_LISTEN` | Override listen address |
| `OC4CLAUDE_UPSTREAM` | Override upstream URL |
| `OC4CLAUDE_LOG_LEVEL` | Log level (debug, info, warn, error) |

### 3.3 Environment Variable Interpolation

Config values support `${VAR}` and `${VAR:-default}` syntax.

## 4. Feature Specifications

### 4.1 Transparent Proxy

- **Behavior:** Proxies all requests from Claude Code without modification to user-visible output
- **Port Handling:** Intercepts HTTP requests to Anthropic endpoints
- **Request Transformation:** Anthropic request → OpenAI request format
- **Response Transformation:** OpenAI response → Anthropic response format
- **Streaming:** Full SSE support with delta transformation

### 4.2 Model Routing

Routes requests to different models based on request characteristics:

| Context | Detection | Default Model |
|---------|-----------|---------------|
| `default` | Standard requests | `claude-3.5-sonnet` |
| `thinking` | Requests with `stream_options.include_usage` or extended thinking | `claude-3.5-haiku` |
| `long_context` | Requests approaching token limit | `claude-3.5-sonnet-20241022` |
| `background` | Background task indicators | `claude-3.5-haiku` |

### 4.3 Fallback Chains

- **Definition:** Ordered list of models to try on failure
- **Behavior:** On API error, automatically retry with next model in chain
- **Circuit Breaker Integration:** Failed models are temporarily skipped
- **Exhausted Chain:** Return error after all models fail

### 4.4 Circuit Breaker

- **State:** CLOSED (normal) → OPEN (failing) → HALF_OPEN (testing)
- **Failure Tracking:** Per-model failure counters
- **Threshold:** Configurable `failure_threshold` (default: 3)
- **Recovery:** After `recovery_timeout_seconds`, one test request is allowed
- **Reset:** Successful request resets failure count

### 4.5 Real-time Streaming

- **Technology:** Server-Sent Events (SSE)
- **Transformations:**
  - OpenAI `data: {"choices":[{"delta":{"content":"..."}}]}` → Anthropic `data: {"type":"content_block_delta","delta":{"type":"text","text":"..."}}`
  - OpenAI `data: {"usage":{...}}` → Anthropic `data: {"type":"message_stop","usage":{...}}`
- **Heartbeat:** Ping/pong handling for keepalive

### 4.6 Tool Calling

Bidirectional translation between formats:

**Anthropic Tool Use → OpenAI Function Call:**
```json
// Anthropic
{"type": "tool_use", "id": "toolu_123", "name": "get_weather", "input": {"location": "NYC"}}

// OpenAI
{"role": "assistant", "tool_calls": [{"id": "toolu_123", "type": "function", "function": {"name": "get_weather", "arguments": "{\"location\":\"NYC\"}"}}]}
```

**Anthropic Tool Result → OpenAI Tool Role:**
```json
// Anthropic
{"type": "tool_result", "tool_use_id": "toolu_123", "content": "Sunny, 72°F"}

// OpenAI
{"role": "tool", "tool_call_id": "toolu_123", "content": "Sunny, 72°F"}
```

### 4.7 Token Counting

- **Encoder:** tiktoken with `cl100k_base` encoding
- **Calculation:** Sum of prompt tokens + expected completion tokens
- **Threshold Detection:** Compare against `token_limit` in config
- **Context Routing:** Switch to `long_context` model when approaching limit

### 4.8 Background Mode

- **Daemon:** Run detached from terminal with PID file
- **PID File:** `~/.oc4claude/oc4claude.pid`
- **Socket File:** `~/.oc4claude/oc4claude.sock` for IPC
- **Signals:** SIGTERM/SIGINT for graceful shutdown
- **Log File:** `~/.oc4claude/oc4claude.log`

### 4.9 Auto-start on Login (Linux/WSL)

- **Method:** `~/.config/autostart/oc4claude.desktop` (XDG compliant)
- **Desktop Entry:** Standard.desktop format with `Hidden=false`
- **WSL Detection:** Check `/proc/version` for WSL signature

## 5. CLI Interface

```
oc4claude [global options] <command> [arguments...]

GLOBAL OPTIONS:
  --config path    Config file path (default: ~/.oc4claude/config.json)
  --verbose        Enable verbose logging

COMMANDS:
  start            Start the proxy server
  stop             Stop the running proxy server
  restart          Restart the proxy server
  status           Show proxy server status
  logs             Show recent logs
  install          Install auto-start on login
  uninstall        Remove auto-start on login
  validate         Validate configuration file

EXAMPLES:
  oc4claude start --verbose
  oc4claude status
  oc4claude install
```

## 6. API Endpoints

### 6.1 Proxy Endpoints (receive Anthropic format)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/messages` | Anthropic Messages API |
| GET | `/v1/messages/streaming` | SSE streaming endpoint |

### 6.2 Health/Status Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (returns `{"status":"ok"}`) |
| GET | `/status` | Detailed status with model health |

## 7. Error Handling

### 7.1 Error Codes

| Code | Description |
|------|-------------|
| `upstream_error` | OpenCode Go API returned error |
| `circuit_open` | All models in circuit breaker state |
| `transform_error` | Request/response transformation failed |
| `timeout` | Request timed out |
| `invalid_config` | Configuration validation failed |

### 7.2 Error Response Format (Anthropic)

```json
{
  "type": "error",
  "error": {
    "type": "upstream_error",
    "message": "OpenAI API error: model not found"
  }
}
```

## 8. File Structure

```
~/.oc4claude/
├── config.json           # Main configuration
├── oc4claude.pid         # PID file (when running)
├── oc4claude.sock        # IPC socket (when running)
├── oc4claude.log         # Log file (when running)
└── oc4claude_desktop.sh  # Autostart helper script
```

## 9. Acceptance Criteria

### 9.1 Core Functionality
- [ ] Claude Code can connect to oc4claude proxy on localhost
- [ ] Anthropic-format requests are transformed to OpenAI format
- [ ] OpenAI responses are transformed back to Anthropic format
- [ ] Streaming responses work in real-time

### 9.2 Model Management
- [ ] Default model routing works based on context
- [ ] Fallback chain retries on failure
- [ ] Circuit breaker skips failing models
- [ ] Token counting detects context limits

### 9.3 Operations
- [ ] Background mode runs as daemon
- [ ] Auto-start installs on Linux/WSL login
- [ ] Graceful shutdown on SIGTERM/SIGINT
- [ ] Configuration reload on SIGHUP

### 9.4 Compatibility
- [ ] Tool use/results translate correctly
- [ ] System prompts preserved
- [ ] Multi-turn conversations maintain context
- [ ] All Anthropic API fields supported

## 10. Technical Stack

- **Language:** Go 1.21+
- **HTTP Framework:** net/http with custom middleware
- **Streaming:** io.Copy with transformation buffers
- **Tokenization:** github.com/pkoukk/tiktoken-go
- **Config:** encoding/json with env interpolation
- **Logging:** log/slog (structured logging)
- **Process Management:** os/exec + signals

## 11. Security Considerations

- API keys stored in config file with appropriate permissions (0600)
- Localhost-only binding by default
- No logging of sensitive request/response content
- Config file path validation
