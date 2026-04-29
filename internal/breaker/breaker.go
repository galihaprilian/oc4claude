package breaker

import (
	"log/slog"
	"sync"
	"time"
)

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "CLOSED"
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

type modelState struct {
	failures    int
	state       State
	lastFailure time.Time
}

type Breaker struct {
	mu                     sync.RWMutex
	models                 map[string]*modelState
	failureThreshold       int
	recoveryTimeout        time.Duration
	recoveryCheckInterval  time.Duration
	stopCh                 chan struct{}
	wg                     sync.WaitGroup
	logger                 *slog.Logger
}

type Config struct {
	FailureThreshold     int
	RecoveryTimeoutSecs int
}

func New(cfg Config, logger *slog.Logger) *Breaker {
	if logger == nil {
		logger = slog.Default()
	}

	b := &Breaker{
		models:                 make(map[string]*modelState),
		failureThreshold:       cfg.FailureThreshold,
		recoveryTimeout:        time.Duration(cfg.RecoveryTimeoutSecs) * time.Second,
		recoveryCheckInterval:  time.Second,
		stopCh:                 make(chan struct{}),
		logger:                 logger,
	}

	b.wg.Add(1)
	go b.recoveryWorker()

	return b
}

func (b *Breaker) GetState(model string) State {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if ms, ok := b.models[model]; ok {
		return ms.state
	}
	return Closed
}

func (b *Breaker) RecordSuccess(model string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ms, ok := b.models[model]
	if !ok {
		ms = &modelState{state: Closed}
		b.models[model] = ms
	}

	if ms.state == HalfOpen {
		b.logger.Info("circuit half_open -> closed",
			"model", model)
	}
	ms.failures = 0
	ms.state = Closed
}

func (b *Breaker) RecordFailure(model string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ms, ok := b.models[model]
	if !ok {
		ms = &modelState{}
		b.models[model] = ms
	}

	ms.failures++
	ms.lastFailure = time.Now()

	if ms.state == HalfOpen {
		b.logger.Warn("circuit half_open -> open (test failed)",
			"model", model)
		ms.state = Open
		return
	}

	if ms.failures >= b.failureThreshold && ms.state == Closed {
		b.logger.Warn("circuit closed -> open",
			"model", model,
			"failures", ms.failures,
			"threshold", b.failureThreshold)
		ms.state = Open
	}
}

func (b *Breaker) IsAvailable(model string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ms, ok := b.models[model]
	if !ok {
		return true
	}

	return ms.state != Open
}

func (b *Breaker) GetAvailableModels(models []string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var available []string
	for _, m := range models {
		if ms, ok := b.models[m]; ok {
			if ms.state != Open {
				available = append(available, m)
			}
		} else {
			available = append(available, m)
		}
	}
	return available
}

func (b *Breaker) GetFailureCount(model string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if ms, ok := b.models[model]; ok {
		return ms.failures
	}
	return 0
}

func (b *Breaker) Close() {
	close(b.stopCh)
	b.wg.Wait()
}

func (b *Breaker) recoveryWorker() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.recoveryCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.checkRecovery()
		}
	}
}

func (b *Breaker) checkRecovery() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for model, ms := range b.models {
		if ms.state == Open {
			if now.Sub(ms.lastFailure) >= b.recoveryTimeout {
				b.logger.Info("circuit open -> half_open (recovery timeout)",
					"model", model,
					"last_failure", ms.lastFailure)
				ms.state = HalfOpen
			}
		}
	}
}
