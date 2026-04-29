# oc4claude User Manual

## Table of Contents

1. [Introduction](#introduction)
2. [Getting Started](#getting-started)
3. [Configuration Guide](#configuration-guide)
4. [Using with Claude Code](#using-with-claude-code)
5. [CLI Reference](#cli-reference)
6. [Model Routing](#model-routing)
7. [Fallback and Circuit Breaker](#fallback-and-circuit-breaker)
8. [Background Mode](#background-mode)
9. [Auto-start on Login](#auto-start-on-login)
10. [API Reference](#api-reference)
11. [Troubleshooting](#troubleshooting)
12. [FAQ](#faq)
13. [Security Considerations](#security-considerations)

---

## Introduction

### What is oc4claude?

oc4claude is a CLI proxy that enables you to use Claude Code with your OpenCode Go subscription. It sits between Claude Code and OpenCode Go, intercepting Anthropic API requests, transforming them to OpenAI format, and forwarding them to OpenCode Go's endpoint.

### How It Works

```
┌─────────────┐    Anthropic API    ┌──────────────┐   OpenAI API   ┌──────────────┐
│ Claude Code │ ◄──────────────────► │  oc4claude   │ ◄────────────► │ OpenCode Go  │
└─────────────┘   (localhost:8080)  └──────────────┘               └──────────────┘
```

Claude Code thinks it's talking to Anthropic, but your requests are routed to affordable OpenAI-compatible models through OpenCode Go.

### Features

- **Transparent Proxy** - Claude Code sends Anthropic-format requests, oc4claude transforms and forwards them
- **Model Routing** - Automatically routes to different models based on context
- **Fallback Chains** - Automatic retry with backup models on failure
- **Circuit Breaker** - Skips failing models to avoid latency
- **Real-time Streaming** - Full SSE streaming with live transformation
- **Tool Calling** - Proper Anthropic ↔ OpenAI function calling translation
- **Token Counting** - Accurate context management with tiktoken
- **Background Mode** - Run as daemon
- **Auto-start** - Launch on system startup (Linux/WSL)

---

## Getting Started

### Prerequisites

- Go 1.21 or later
- OpenCode Go subscription with API key
- Claude Code installed

### Installation

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/galihaprilian/oc4claude.git
cd oc4claude

# Build
go build -o oc4claude ./cmd/oc4claude

# Or install globally
go install ./cmd/oc4claude
```

### First-Time Setup

1. **Create configuration directory:**
   ```bash
   mkdir -p ~/.oc4claude
   ```

2. **Set your API key:**
   ```bash
   export OPENCODE_API_KEY="your-opencode-api-key"
   ```

3. **Start the proxy:**
   ```bash
   ./oc4claude start
   ```

4. **Verify it's running:**
   ```bash
   ./oc4claude status
   ```

---

## Configuration Guide

### Configuration File

Location: `~/.oc4claude/config.json`

#### Full Configuration Example

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

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `listen` | string | `127.0.0.1:8080` | Address to listen on |
| `upstream_url` | string | `https://api.opencode.ai/v1` | OpenCode Go API endpoint |
| `api_key` | string | - | OpenCode Go API key (use env var) |
| `default_model` | string | `anthropic/claude-3.5-sonnet` | Default model for requests |
| `models` | object | (see above) | Model mapping by context |
| `fallback_chain` | array | (see above) | Ordered list of fallback models |
| `circuit_breaker.enabled` | boolean | `true` | Enable circuit breaker |
| `circuit_breaker.failure_threshold` | int | `3` | Failures before opening circuit |
| `circuit_breaker.recovery_timeout_seconds` | int | `60` | Seconds before trying again |
| `token_limit` | int | `180000` | Token limit for context detection |
| `log_level` | string | `info` | Log level: debug, info, warn, error |
| `background` | boolean | `false` | Run in background by default |
| `auto_start` | boolean | `false` | Auto-start on system login |

### Environment Variable Interpolation

Config values support `${VAR}` and `${VAR:-default}` syntax:

```json
{
  "api_key": "${OPENCODE_API_KEY}",
  "listen": "${OC4CLAUDE_LISTEN:-127.0.0.1:8080}"
}
```

### Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `OPENCODE_API_KEY` | OpenCode Go API key |
| `OC4CLAUDE_LISTEN` | Override listen address |
| `OC4CLAUDE_UPSTREAM` | Override upstream URL |
| `OC4CLAUDE_LOG_LEVEL` | Log level (debug, info, warn, error) |

---

## Using with Claude Code

### Step 1: Start oc4claude Proxy

```bash
# Set your API key
export OPENCODE_API_KEY="sk-..."

# Start the proxy
./oc4claude start

# Verify it's running
./oc4claude status
```

### Step 2: Configure Claude Code

Set environment variables before running Claude Code:

```bash
# Required - can be any value, Claude Code just needs to think it's talking to Anthropic
export ANTHROPIC_API_KEY="anything-here"

# Point to our proxy instead of Anthropic
export ANTHROPIC_BASE_URL="http://localhost:8080"

# Start Claude Code
claude
```

### Step 3: Verify Connection

Check the proxy status:

```bash
./oc4claude status
```

Should show:
```
Status: Running
PID: 12345
Listen: 127.0.0.1:8080
Upstream: https://api.opencode.ai/v1
```

### Alternative: Claude Code Configuration File

You can also configure Claude Code via its config file:

```json
{
  "api_key": "dummy-key",
  "base_url": "http://localhost:8080"
}
```

### Using with claude Code CLI

```bash
# Full command with env vars
ANTHROPIC_API_KEY=dummy ANTHROPIC_BASE_URL=http://localhost:8080 claude "Your prompt here"
```

---

## CLI Reference

### Global Flags

```
--config path    Config file path (default: ~/.oc4claude/config.json)
--verbose        Enable verbose logging (debug level)
```

### Commands

#### start

Start the proxy server.

```bash
oc4claude start [flags]
```

**Flags:**
- `--background, -b` - Run in background (daemon mode)

**Example:**
```bash
# Start in foreground
oc4claude start

# Start in background
oc4claude start --background

# Start with verbose logging
oc4claude start --verbose
```

#### stop

Stop the running proxy server.

```bash
oc4claude stop
```

**Example:**
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

**Output:**
```
Status: Running
PID: 12345
Listen: 127.0.0.1:8080
Upstream: https://api.opencode.ai/v1
Model: anthropic/claude-3.5-sonnet
Uptime: 2 hours
```

#### logs

Show recent logs.

```bash
oc4claude logs [flags]
```

**Flags:**
- `--lines, -n` - Number of lines to show (default: 50)

**Example:**
```bash
# Show last 50 lines
oc4claude logs

# Show last 100 lines
oc4claude logs --lines 100
```

#### install

Install auto-start on login (Linux/WSL only).

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

**Example:**
```bash
# Validate default config
oc4claude validate

# Validate custom config
oc4claude validate --config /path/to/config.json
```

---

## Model Routing

### Context Types

oc4claude automatically routes requests to different models based on context:

| Context | Detection | Default Model | Use Case |
|---------|-----------|---------------|----------|
| `default` | Standard requests | claude-3.5-sonnet | General tasks |
| `thinking` | Extended thinking enabled | claude-3.5-haiku | Quick reasoning |
| `long_context` | Approaching token limit | claude-3.5-sonnet-20241022 | Large contexts |
| `background` | Background task indicators | claude-3.5-haiku | Non-urgent tasks |

### How Routing Works

1. Request arrives at proxy
2. Router analyzes request characteristics:
   - Token count (approaching limit → long_context)
   - Stream options (extended thinking → thinking)
   - Request metadata (background flags)
3. Selects appropriate model from config
4. Forwards to OpenCode Go with selected model

### Customizing Model Mapping

Edit `~/.oc4claude/config.json`:

```json
{
  "models": {
    "default": "anthropic/claude-3.5-sonnet",
    "thinking": "anthropic/claude-3.5-haiku",
    "long_context": "anthropic/claude-3.5-sonnet-20241022",
    "background": "anthropic/claude-3.5-haiku"
  }
}
```

---

## Fallback and Circuit Breaker

### Fallback Chain

When a model fails, oc4claude automatically tries the next model in your configured chain.

**Configuration:**
```json
{
  "fallback_chain": [
    "anthropic/claude-3.5-sonnet",
    "anthropic/claude-3.5-haiku",
    "anthropic/claude-opus"
  ]
}
```

**Behavior:**
1. Request to primary model fails
2. Try next model in chain
3. Repeat until success or chain exhausted
4. Return error if all models fail

### Circuit Breaker

The circuit breaker prevents repeated calls to failing models.

**States:**
- **CLOSED** - Normal operation, requests flow through
- **OPEN** - Model is failing, requests skip to next in chain
- **HALF_OPEN** - Testing if model recovered

**Configuration:**
```json
{
  "circuit_breaker": {
    "enabled": true,
    "failure_threshold": 3,
    "recovery_timeout_seconds": 60
  }
}
```

**Behavior:**
1. Model fails `failure_threshold` times → Circuit OPEN
2. Wait `recovery_timeout_seconds`
3. Allow one test request → Circuit HALF_OPEN
4. If test succeeds → Circuit CLOSED
5. If test fails → Circuit OPEN again

### Checking Model Health

```bash
curl http://localhost:8080/status
```

Returns detailed status including model health.

---

## Background Mode

### Running as Daemon

```bash
# Start in background
oc4claude start --background

# Check status
oc4claude status

# Stop
oc4claude stop
```

### File Locations

When running in background:

| File | Location |
|------|----------|
| PID file | `~/.oc4claude/oc4claude.pid` |
| Log file | `~/.oc4claude/oc4claude.log` |
| Socket | `~/.oc4claude/oc4claude.sock` |

### Log File

All logs are written to `~/.oc4claude/oc4claude.log`:

```bash
# View logs
oc4claude logs

# Follow logs in real-time
tail -f ~/.oc4claude/oc4claude.log
```

### Graceful Shutdown

```bash
# Stop gracefully
oc4claude stop

# Or send SIGTERM to PID
kill $(cat ~/.oc4claude/oc4claude.pid)
```

---

## Auto-start on Login

### Installing Auto-start

```bash
# Install (Linux/WSL)
oc4claude install

# Verify
ls ~/.config/autostart/oc4claude.desktop
```

### Desktop Entry

Creates `~/.config/autostart/oc4claude.desktop`:

```desktop
[Desktop Entry]
Type=Application
Name=oc4claude
Exec=/path/to/oc4claude start
Hidden=false
X-GNOME-Autostart-enabled=true
```

### Removing Auto-start

```bash
oc4claude uninstall
```

### WSL Support

Auto-start detection checks for WSL in `/proc/version` and adapts accordingly.

---

## API Reference

### Proxy Endpoints

Claude Code sends requests to these endpoints:

#### POST /v1/messages

Anthropic Messages API.

**Request:**
```json
{
  "model": "claude-3.5-sonnet",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "max_tokens": 1024
}
```

**Response:**
```json
{
  "id": "msg_xxx",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "Hello!"}
  ],
  "model": "claude-3.5-sonnet"
}
```

#### GET /v1/messages/streaming

SSE streaming endpoint.

**Response:** Server-Sent Events stream with chunks:
```
data: {"type":"content_block_delta","index":0,"delta":{"type":"text","text":"Hello"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"text","text":"!"}}

data: {"type":"message_stop"}
```

### Health Endpoints

#### GET /health

Simple health check.

**Response:**
```json
{"status":"ok"}
```

#### GET /status

Detailed status with model health.

**Response:**
```json
{
  "status": "running",
  "uptime": "2h30m",
  "listen": "127.0.0.1:8080",
  "upstream": "https://api.opencode.ai/v1",
  "models": {
    "anthropic/claude-3.5-sonnet": {"failures": 0, "circuit": "closed"},
    "anthropic/claude-3.5-haiku": {"failures": 1, "circuit": "half_open"}
  }
}
```

---

## Troubleshooting

### Connection Issues

**Problem: Claude Code can't connect to proxy**

```bash
# Check if proxy is running
oc4claude status

# Check if port is listening
ss -tlnp | grep 8080

# Check logs for errors
oc4claude logs
```

**Solution:** Start the proxy if not running:
```bash
oc4claude start
```

### Authentication Errors

**Problem: "API key required" or authentication failures**

**Solution:** Ensure `OPENCODE_API_KEY` is set:
```bash
export OPENCODE_API_KEY="your-actual-api-key"
oc4claude stop
oc4claude start
```

### Streaming Problems

**Problem: Responses are slow or streaming breaks**

**Possible causes:**
- Network issues to OpenCode Go
- Model is overloaded
- Circuit breaker is open

**Solution:** Check status and logs:
```bash
curl http://localhost:8080/status
oc4claude logs | grep -i error
```

### Circuit Breaker Issues

**Problem: All models show as unavailable**

**Solution:** Wait for recovery timeout, or restart:
```bash
oc4claude stop
oc4claude start
```

### Port Already in Use

**Problem: "Address already in use"**

**Solution:** Find and stop existing process:
```bash
# Find process using port 8080
lsof -i :8080

# Or check PID file
cat ~/.oc4claude/oc4claude.pid

# Stop the existing instance
oc4claude stop
```

### Log Analysis

```bash
# All logs
oc4claude logs

# Only errors
oc4claude logs | grep -i error

# Follow in real-time
tail -f ~/.oc4claude/oc4claude.log
```

---

## FAQ

### Q: Do I need an Anthropic API key?

A: No. Claude Code needs to think it's talking to Anthropic, but you can use any dummy value for `ANTHROPIC_API_KEY`. The actual authentication happens at OpenCode Go.

### Q: How is this different from using Claude Code directly with Anthropic?

A: oc4claude routes your requests through OpenCode Go, which provides access to OpenAI-compatible models at different pricing. Claude Code works normally - it just doesn't know it's talking to a different backend.

### Q: Does streaming work?

A: Yes. Full SSE streaming is supported with real-time transformation of chunks.

### Q: Can I use my own model names?

A: Yes. The `models` config maps context types to model identifiers. You can use any model that OpenCode Go supports.

### Q: What happens if all fallback models fail?

A: After exhausting the fallback chain, oc4claude returns an error response to Claude Code. The error will indicate which models failed.

### Q: How do I update the config without restarting?

A: Currently, config changes require restart:
```bash
oc4claude stop
# Edit ~/.oc4claude/config.json
oc4claude start
```

### Q: Can I run on a different port?

A: Yes. Set `listen` in config or use environment variable:
```bash
export OC4CLAUDE_LISTEN="127.0.0.1:9000"
```

### Q: Does this work on macOS or Windows?

A: Currently Linux and WSL are supported. Auto-start is Linux/WSL only.

---

## Security Considerations

### API Key Storage

- Store your `OPENCODE_API_KEY` securely
- Config file permissions should be restricted: `chmod 600 ~/.oc4claude/config.json`
- Consider using environment variables instead of config file

### Network Security

- Default binding is `127.0.0.1` (localhost only)
- Only expose the proxy on localhost unless you need remote access
- For remote access, consider using a reverse proxy with TLS

### Logging

- Logs may contain request metadata
- Avoid logging sensitive content in production
- Rotate log files regularly

### Permissions

The proxy runs with the permissions of the user who starts it. Ensure appropriate file permissions on:
- Config file (`~/.oc4claude/config.json`)
- PID file (`~/.oc4claude/oc4claude.pid`)
- Log file (`~/.oc4claude/oc4claude.log`)

---

## Appendix: Complete Configuration Reference

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

### Default Values

| Option | Default |
|--------|---------|
| listen | `127.0.0.1:8080` |
| upstream_url | `https://api.opencode.ai/v1` |
| default_model | `anthropic/claude-3.5-sonnet` |
| circuit_breaker.enabled | `true` |
| circuit_breaker.failure_threshold | `3` |
| circuit_breaker.recovery_timeout_seconds | `60` |
| token_limit | `180000` |
| log_level | `info` |
| background | `false` |
| auto_start | `false` |

---

*Document Version: 1.0*
*Last Updated: 2026-04-29*
