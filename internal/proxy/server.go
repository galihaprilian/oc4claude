package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/galihaprilian/oc4claude/internal/breaker"
	"github.com/galihaprilian/oc4claude/internal/config"
	"github.com/galihaprilian/oc4claude/internal/router"
	"github.com/galihaprilian/oc4claude/internal/transform"
	"github.com/galihaprilian/oc4claude/pkg/anthropic"
	"github.com/galihaprilian/oc4claude/pkg/openai"
)

type Server struct {
	config      *config.Config
	router      *router.Router
	breaker     *breaker.Breaker
	upstreamURL string
	httpServer  *http.Server
	logger      *slog.Logger
}

func New(cfg *config.Config, router *router.Router, breaker *breaker.Breaker, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{
		config:      cfg,
		router:      router,
		breaker:     breaker,
		upstreamURL: cfg.UpstreamURL,
		logger:      logger,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/messages", s.handleMessages)
	mux.HandleFunc("GET /v1/messages/streaming", s.handleStreaming)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /status", s.handleStatus)

	s.httpServer = &http.Server{
		Addr:         s.config.Listen,
		Handler:     mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.logger.Info("starting proxy server", "addr", s.config.Listen)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	var anthropicReq anthropic.MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	defer r.Body.Close()

	tokenCount := estimateTokenCount(&anthropicReq)
	model := s.router.GetModel(r.Context(), &anthropicReq, tokenCount)

	fallbacks := s.getFallbacks(model)
	availableModels := s.breaker.GetAvailableModels(fallbacks)
	if len(availableModels) == 0 {
		availableModels = fallbacks
	}

	var lastErr error
	for _, targetModel := range availableModels {
		if s.breaker != nil && !s.breaker.IsAvailable(targetModel) {
			continue
		}

		result, err := s.forwardToUpstream(r.Context(), &anthropicReq, targetModel)
		if err != nil {
			lastErr = err
			s.handleModelError(targetModel, err)
			continue
		}

		if s.breaker != nil {
			s.breaker.RecordSuccess(targetModel)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
		return
	}

	if lastErr != nil {
		s.writeError(w, http.StatusBadGateway, "upstream_error", lastErr.Error())
		return
	}

	s.writeError(w, http.StatusServiceUnavailable, "circuit_open", "all models are unavailable")
}

func (s *Server) handleStreaming(w http.ResponseWriter, r *http.Request) {
	var anthropicReq anthropic.MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&anthropicReq); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	defer r.Body.Close()

	anthropicReq.Stream = true

	tokenCount := estimateTokenCount(&anthropicReq)
	model := s.router.GetModel(r.Context(), &anthropicReq, tokenCount)

	fallbacks := s.getFallbacks(model)
	availableModels := s.breaker.GetAvailableModels(fallbacks)
	if len(availableModels) == 0 {
		availableModels = fallbacks
	}

	var lastErr error
	for _, targetModel := range availableModels {
		if s.breaker != nil && !s.breaker.IsAvailable(targetModel) {
			continue
		}

		err := s.forwardStreaming(r.Context(), w, &anthropicReq, targetModel)
		if err != nil {
			lastErr = err
			s.handleModelError(targetModel, err)
			continue
		}

		if s.breaker != nil {
			s.breaker.RecordSuccess(targetModel)
		}
		return
	}

	if lastErr != nil {
		s.writeStreamingError(w, http.StatusBadGateway, "upstream_error", lastErr.Error())
		return
	}

	s.writeStreamingError(w, http.StatusServiceUnavailable, "circuit_open", "all models are unavailable")
}

func (s *Server) forwardToUpstream(ctx context.Context, anthropicReq *anthropic.MessageRequest, targetModel string) (*anthropic.MessageResponse, error) {
	openaiReq, err := transform.TransformMessageRequest(anthropicReq, targetModel)
	if err != nil {
		return nil, fmt.Errorf("transform error: %w", err)
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.upstreamURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var openaiResp openai.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	anthropicResp, err := transform.TransformResponse(&openaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to transform response: %w", err)
	}

	return anthropicResp, nil
}

func (s *Server) forwardStreaming(ctx context.Context, w http.ResponseWriter, anthropicReq *anthropic.MessageRequest, targetModel string) error {
	openaiReq, err := transform.TransformMessageRequest(anthropicReq, targetModel)
	if err != nil {
		return fmt.Errorf("transform error: %w", err)
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.upstreamURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming not supported")
	}

	transformer := transform.NewStreamTransformer(resp.Body)

	buf := make([]byte, 4096)
	for {
		n, err := transformer.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write to client: %w", writeErr)
			}
			flusher.Flush()
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("stream read error: %w", err)
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status": "ok",
		"models": map[string]interface{}{},
	}

	if s.breaker != nil {
		modelStates := make(map[string]interface{})
		for _, model := range s.config.FallbackChain {
			state := s.breaker.GetState(model)
			modelStates[model] = map[string]interface{}{
				"state":         state.String(),
				"failures":      s.breaker.GetFailureCount(model),
				"available":     s.breaker.IsAvailable(model),
			}
		}
		if s.config.DefaultModel != "" {
			state := s.breaker.GetState(s.config.DefaultModel)
			modelStates[s.config.DefaultModel] = map[string]interface{}{
				"state":         state.String(),
				"failures":      s.breaker.GetFailureCount(s.config.DefaultModel),
				"available":     s.breaker.IsAvailable(s.config.DefaultModel),
			}
		}
		status["models"] = modelStates
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResp := anthropic.ErrorResponse{
		Type: "error",
		Error: anthropic.Error{
			Type:    code,
			Message: message,
		},
	}
	json.NewEncoder(w).Encode(errorResp)
}

func (s *Server) writeStreamingError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(status)
	errorResp := anthropic.ErrorResponse{
		Type: "error",
		Error: anthropic.Error{
			Type:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(errorResp)
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (s *Server) handleModelError(model string, err error) {
	if s.breaker != nil {
		s.breaker.RecordFailure(model)
	}
	s.logger.Error("model request failed",
		"model", model,
		"error", err.Error(),
	)
}

func (s *Server) getFallbacks(primary string) []string {
	fallbacks := make([]string, 0, len(s.config.FallbackChain)+1)
	fallbacks = append(fallbacks, primary)
	for _, m := range s.config.FallbackChain {
		if m != primary {
			fallbacks = append(fallbacks, m)
		}
	}
	return fallbacks
}

func estimateTokenCount(req *anthropic.MessageRequest) int {
	if req == nil {
		return 0
	}

	total := 0
	for _, msg := range req.Messages {
		if content, ok := msg.Content.(string); ok {
			total += len(content) / 4
		}
	}
	if req.System != "" {
		total += len(req.System) / 4
	}
	return total
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func mapErrorToAnthropic(err error) (string, string) {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "TimedOut") {
		return "timeout", "request timed out"
	}

	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "refused") {
		return "connection_error", "failed to connect to upstream"
	}

	if strings.Contains(errStr, "circuit") || strings.Contains(errStr, "unavailable") {
		return "circuit_open", "model circuit breaker is open"
	}

	if strings.Contains(errStr, "rate") || strings.Contains(errStr, "quota") {
		return "rate_limit", "rate limit exceeded"
	}

	return "api_error", errStr
}
