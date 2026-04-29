# oc4claude

[![Go Version](https://img.shields.io/badge/Go-1.26.2+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

**oc4claude** is a CLI proxy that intercepts Anthropic API requests from Claude Code, transforms them to OpenAI format, forwards them to OpenCode Go's endpoint, and transforms responses back to Anthropic format.

## Overview

oc4claude enables developers to use Claude Code with an OpenCode Go subscription for cost-effective AI assistance. It acts as a transparent middleware that:

- Listens locally for Claude Code's Anthropic API requests
- Transforms requests from Anthropic format to OpenAI format
- Forwards requests to OpenCode Go's API
- Transforms responses back to Anthropic format for Claude Code

### Architecture

```
┌─────────────┐    Anthropic API    ┌──────────────┐   OpenAI API   ┌──────────────┐
│ Claude Code │ ◄──────────────────► │  oc4claude   │ ◄────────────► │ OpenCode Go  │
└─────────────┘   (localhost:8080)  └──────────────┘               └──────────────┘
```

## Features

### Transparent Proxy
Proxies all requests from Claude Code without modification to user-visible output. Handles HTTP requests to Anthropic endpoints with full request/response transformation.

### Model Routing
Automatically routes requests to different models based on request characteristics:
- **default**: Standard requests
- **thinking**: Requests with extended thinking enabled
- **long_context**: Requests approaching token limits
- **background**: Background task indicators

### Fallback Chains
Ordered list of models to try on failure. Automatically retries with the next model in the chain when an API error occurs.

### Circuit Breaker
Prevents repeated requests to failing models:
- **States**: CLOSED (normal) → OPEN (failing) → HALF_OPEN (testing)
- **Recovery**: After timeout, allows one test request
- **Reset**: Successful requests reset failure count

### Real-time Streaming
Full Server-Sent Events (SSE) support with delta transformation:
- OpenAI `data: {"choices":[{"delta":{"content":"..."}}]}` → Anthropic `data: {"type":"content_block_delta","delta":{"type":"text","text":"..."}}`
- Proper ping/pong keepalive handling

### Tool Calling
Bidirectional translation between Anthropic and OpenAI formats:
- Anthropic `tool_use` ↔ OpenAI `tool_calls`
- Anthropic `tool_result` ↔ OpenAI `tool` role

### Token Counting
Uses tiktoken with `cl100k_base` encoding to estimate token count and route to appropriate models based on context limits.

### Background Mode
Run as a detached daemon with:
- PID file (`~/.oc4claude/oc4claude.pid`)
- Unix socket IPC (`~/.oc4claude/oc4claude.sock`)
- Log file (`~/.oc4claude/oc4claude.log`)
- Graceful shutdown on SIGTERM/SIGINT

### Auto-start on Login
Installs auto-start on Linux/WSL systems via `~/.config/autostart/oc4claude.desktop`.

## Quick Start

### 1. Download or Build

```bash
# Build from source
git clone https://github.com/galihaprilian/oc4claude.git
cd oc4claude
go build -o oc4claude ./cmd/oc4claude

# Or install directly
go install github.com/galihaprilian/oc4claude/cmd/oc4claude@latest
```

### 2. Configure

Create `~/.oc4claude/config.json`:

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
    "anthropic/claude-3.5-haiku"
  ],
  "circuit_breaker": {
    "enabled": true,
    "failure_threshold": 3,
    "recovery_timeout_seconds": 60
  },
  "token_limit": 180000,
  "log_level": "info"
}
```

Set your API key:
```bash
export OPENCODE_API_KEY="your-opencode-api-key"
```

### 3. Start the Proxy

```bash
# Foreground (for testing)
oc4claude start --verbose

# Background mode
oc4claude start --background
```

### 4. Configure Claude Code

Set the environment variable:
```bash
export ANTHROPIC_API_KEY="anything"  # Claude Code requires this to be set
export ANTHROPIC_BASE_URL="http://localhost:8080"
```

Or use Claude Code's configuration:
```bash
claude config set api_key "anything"
claude config set base_url "http://localhost:8080"
```

## Installation

### Build from Source

**Requirements:**
- Go 1.21+
- Git

```bash
# Clone the repository
git clone https://github.com/galihaprilian/oc4claude.git
cd oc4claude

# Build
go build -o oc4claude ./cmd/oc4claude

# Install (optional)
sudo mv oc4claude /usr/local/bin/
```

### Verify Installation

```bash
oc4claude validate
```

## Configuration

### Config File Location

Default: `~/.oc4claude/config.json`

### Configuration Reference

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
    "anthropic/claude-3.5-haiku"
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

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `listen` | string | `127.0.0.1:8080` | Address to listen on |
| `upstream_url` | string | `https://api.opencode.ai/v1` | OpenCode Go API endpoint |
| `api_key` | string | (required) | OpenCode Go API key |
| `default_model` | string | `anthropic/claude-3.5-sonnet` | Default model for routing |
| `models` | object | (see above) | Context-specific model mappings |
| `fallback_chain` | array | (see above) | Ordered fallback models |
| `circuit_breaker.enabled` | bool | `true` | Enable circuit breaker |
| `circuit_breaker.failure_threshold` | int | `3` | Failures before opening |
| `circuit_breaker.recovery_timeout_seconds` | int | `60` | Recovery timeout |
| `token_limit` | int | `180000` | Token limit for context routing |
| `log_level` | string | `info` | Log level (debug, info, warn, error) |
| `background` | bool | `false` | Run in background mode |
| `auto_start` | bool | `false` | Auto-start on login |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENCODE_API_KEY` | OpenCode Go API key |
| `OC4CLAUDE_LISTEN` | Override listen address |
| `OC4CLAUDE_UPSTREAM` | Override upstream URL |
| `OC4CLAUDE_LOG_LEVEL` | Log level (debug, info, warn, error) |

### Environment Variable Interpolation

Config values support `${VAR}` and `${VAR:-default}` syntax:

```json
{
  "api_key": "${OPENCODE_API_KEY}",
  "listen": "${OC4CLAUDE_LISTEN:-127.0.0.1:8080}"
}
```

## CLI Commands

```
oc4claude [global options] <command> [arguments...]
```

### Global Options

| Option | Description |
|--------|-------------|
| `--config path` | Config file path (default: `~/.oc4claude/config.json`) |
| `--verbose` | Enable verbose logging |

### Commands

#### start
Start the proxy server.

```bash
# Start in foreground
oc4claude start

# Start with verbose logging
oc4claude start --verbose

# Start in background
oc4claude start --background
```

#### stop
Stop the running proxy server.

```bash
oc4claude stop
```

#### restart
Restart the proxy server.

```bash
oc4claude restart
```

#### status
Show proxy server status.

```bash
oc4claude status
```

#### logs
Show recent logs.

```bash
oc4claude logs
```

#### install
Install auto-start on login.

```bash
oc4claude install
```

#### uninstall
Remove auto-start on login.

```bash
oc4claude uninstall
```

#### validate
Validate configuration file.

```bash
oc4claude validate
```

## Claude Code Setup

### Method 1: Environment Variables

```bash
# Set required environment variables
export ANTHROPIC_API_KEY="anything"  # Must be set for Claude Code to work
export ANTHROPIC_BASE_URL="http://localhost:8080"

# Start Claude Code
claude
```

### Method 2: Claude Code Configuration

```bash
# Configure Claude Code to use oc4claude
claude config set api_key "anything"
claude config set base_url "http://localhost:8080"

# Verify configuration
claude config get
```

### Verify Connection

```bash
# Check oc4claude status
curl http://localhost:8080/health
# Returns: {"status":"ok"}

# Check model status
curl http://localhost:8080/status
```

## Architecture

### Components

```
oc4claude/
├── cmd/oc4claude/         # CLI entry point
│   ├── main.go
│   └── install.go         # Auto-start installation
├── internal/
│   ├── proxy/server.go    # HTTP server handling requests
│   ├── router/router.go   # Model routing logic
│   ├── breaker/breaker.go # Circuit breaker implementation
│   ├── config/config.go   # Configuration management
│   ├── tokenizer/          # Token counting
│   └── transform/         # Request/response transformation
│       ├── request.go     # Anthropic → OpenAI
│       ├── response.go    # OpenAI → Anthropic
│       └── stream.go      # Streaming transformation
└── pkg/
    ├── anthropic/types.go # Anthropic API types
    └── openai/types.go    # OpenAI API types
```

### Request Flow

1. Claude Code sends request to `localhost:8080` (Anthropic format)
2. Proxy receives and decodes the request
3. Router determines appropriate model based on context
4. Request transformer converts Anthropic → OpenAI format
5. Request forwarded to OpenCode Go API
6. Response transformer converts OpenAI → Anthropic format
7. Response returned to Claude Code

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/messages` | Anthropic Messages API |
| GET | `/v1/messages/streaming` | SSE streaming endpoint |
| GET | `/health` | Health check |
| GET | `/status` | Detailed status with model health |

## Troubleshooting

### Connection Refused

```
Error: dial tcp 127.0.0.1:8080: connection refused
```

**Solution**: Ensure oc4claude is running:
```bash
oc4claude start
```

### Circuit Breaker Open

```
Error: all models are unavailable
```

**Solution**: Check model status and wait for recovery:
```bash
curl http://localhost:8080/status
```

### Invalid API Key

```
Error: upstream returned status 401
```

**Solution**: Verify your OpenCode Go API key is set correctly:
```bash
export OPENCODE_API_KEY="your-valid-api-key"
oc4claude restart
```

### Token Limit Errors

If you receive context length errors:
- Increase `token_limit` in config
- Ensure `long_context` model is properly configured

### View Logs

```bash
# If running in background
oc4claude logs

# Or check the log file directly
cat ~/.oc4claude/oc4claude.log
```

### Debug Mode

Run with verbose logging:
```bash
oc4claude start --verbose
```

### Reset Circuit Breaker

Restart the proxy to reset all circuit breaker states:
```bash
oc4claude restart
```

## File Structure

```
~/.oc4claude/
├── config.json           # Main configuration
├── oc4claude.pid         # PID file (when running)
├── oc4claude.sock        # IPC socket (when running)
├── oc4claude.log         # Log file (when running)
└── oc4claude.desktop     # Auto-start desktop entry
```

## License

MIT License

Copyright (c) 2024

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.