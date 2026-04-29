package router

import (
	"context"
	"strings"

	"github.com/galihaprilian/oc4claude/internal/breaker"
	"github.com/galihaprilian/oc4claude/internal/config"
)

type Router struct {
	config      *config.Config
	breaker     *breaker.Breaker
	modelConfig map[string]string
}

func New(cfg *config.Config, br *breaker.Breaker) *Router {
	return &Router{
		config:      cfg,
		breaker:     br,
		modelConfig: cfg.Models,
	}
}

func (r *Router) GetModel(ctx context.Context, req interface{}, tokenCount int) string {
	contextType := r.DetectContext(req)

	model, ok := r.modelConfig[contextType]
	if !ok {
		model = r.config.DefaultModel
	}

	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}

	availableModel := model
	if r.breaker != nil && !r.breaker.IsAvailable(model) {
		fallbacks := r.getFallbacks(model)
		availableModels := r.breaker.GetAvailableModels(fallbacks)
		if len(availableModels) > 0 {
			availableModel = availableModels[0]
		} else {
			availableModel = model
		}
	}

	return r.MapModel(availableModel)
}

func (r *Router) DetectContext(req interface{}) string {
	switch v := req.(type) {
	case interface{ GetStreamOptions() *StreamOptions }:
		if v.GetStreamOptions() != nil && v.GetStreamOptions().IncludeUsage {
			return "thinking"
		}
	case interface{ IsBackgroundTask() bool }:
		if v.IsBackgroundTask() {
			return "background"
		}
	case interface{ GetMaxTokens() int }:
		if v.GetMaxTokens() > 100000 {
			return "long_context"
		}
	}

	if r.isLongContext(req) {
		return "long_context"
	}

	if r.isBackgroundRequest(req) {
		return "background"
	}

	if r.hasThinkingRequest(req) {
		return "thinking"
	}

	return "default"
}

func (r *Router) MapModel(model string) string {
	mapping := map[string]string{
		"anthropic/claude-3.5-sonnet":       "openai/gpt-4o",
		"anthropic/claude-3.5-haiku":         "openai/gpt-4o-mini",
		"anthropic/claude-3.5-sonnet-20241022": "openai/gpt-4o-2024-10-22",
		"anthropic/claude-opus":              "openai/gpt-4-turbo",
	}

	if mapped, ok := mapping[model]; ok {
		return mapped
	}

	if strings.HasPrefix(model, "anthropic/") {
		return strings.Replace(model, "anthropic/", "openai/", 1)
	}

	return model
}

func (r *Router) getFallbacks(primary string) []string {
	fallbacks := make([]string, 0, len(r.config.FallbackChain)+1)
	fallbacks = append(fallbacks, primary)
	for _, m := range r.config.FallbackChain {
		if m != primary {
			fallbacks = append(fallbacks, m)
		}
	}
	return fallbacks
}

func (r *Router) isLongContext(req interface{}) bool {
	return false
}

func (r *Router) isBackgroundRequest(req interface{}) bool {
	return false
}

func (r *Router) hasThinkingRequest(req interface{}) bool {
	return false
}

type StreamOptions struct {
	IncludeUsage bool
}
