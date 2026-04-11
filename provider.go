// Package openfeaturego provides an OpenFeature provider that wraps the
// github.com/quonfig/sdk-go native SDK.
//
// Usage:
//
//	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
//	    SDKKey:              "qf_sk_...",
//	    TargetingKeyMapping: "user.id", // default
//	})
//	openfeature.SetProviderAndWait(provider)
//	client := openfeature.NewDefaultClient()
//	enabled, _ := client.BooleanValue(ctx, "my-flag", false, evalCtx)
package openfeaturego

import (
	"context"
	"sync"

	"github.com/open-feature/go-sdk/openfeature"
	quonfig "github.com/quonfig/sdk-go"
)

// Options configures the QuonfigProvider.
type Options struct {
	// SDKKey is the Quonfig API key (e.g. "qf_sk_production_...").
	// Mutually exclusive with DataDir.
	SDKKey string

	// DataDir sets the local Quonfig workspace directory (for offline/test mode).
	// Mutually exclusive with SDKKey.
	DataDir string

	// Environment selects which environment to evaluate (e.g. "Production").
	Environment string

	// TargetingKeyMapping specifies how the OpenFeature targetingKey maps to a
	// Quonfig context property. Dot-notation: "user.id" means namespace "user",
	// property "id". Defaults to "user.id".
	TargetingKeyMapping string

	// AdditionalOptions are extra quonfig.Option values passed through to NewClient.
	AdditionalOptions []quonfig.Option
}

// QuonfigProvider implements the OpenFeature FeatureProvider and StateHandler interfaces.
// It wraps the github.com/quonfig/sdk-go native client.
type QuonfigProvider struct {
	opts        Options
	client      *quonfig.Client
	mu          sync.RWMutex
	ready       bool // set to true after Init() completes successfully
	eventCh     chan openfeature.Event
}

// NewQuonfigProvider constructs a new QuonfigProvider. Call openfeature.SetProviderAndWait
// to register it; that will call Init() to initialize the underlying Quonfig client.
func NewQuonfigProvider(opts Options) *QuonfigProvider {
	if opts.TargetingKeyMapping == "" {
		opts.TargetingKeyMapping = "user.id"
	}
	return &QuonfigProvider{
		opts:    opts,
		eventCh: make(chan openfeature.Event, 16),
	}
}

// Metadata returns the provider metadata.
func (p *QuonfigProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: "quonfig"}
}

// Hooks returns the provider-level hooks (none for this provider).
func (p *QuonfigProvider) Hooks() []openfeature.Hook {
	return nil
}

// EventChannel returns the channel on which this provider emits lifecycle events.
// Implements openfeature.EventHandler.
func (p *QuonfigProvider) EventChannel() <-chan openfeature.Event {
	return p.eventCh
}

// Init initializes the underlying Quonfig client. Called by the OpenFeature SDK
// when the provider is registered via SetProvider / SetProviderAndWait.
// Implements openfeature.StateHandler.
func (p *QuonfigProvider) Init(_ openfeature.EvaluationContext) error {
	quonfigOpts := p.buildQuonfigOptions()

	client, err := quonfig.NewClient(quonfigOpts...)
	if err != nil {
		p.sendEvent(openfeature.ProviderError, openfeature.ProviderEventDetails{
			Message: err.Error(),
		})
		return err
	}

	p.mu.Lock()
	p.client = client
	p.ready = true
	p.mu.Unlock()

	p.sendEvent(openfeature.ProviderReady, openfeature.ProviderEventDetails{})
	return nil
}

// Shutdown closes the underlying Quonfig client.
// Implements openfeature.StateHandler.
func (p *QuonfigProvider) Shutdown() {
	p.mu.RLock()
	client := p.client
	p.mu.RUnlock()
	if client != nil {
		client.Close()
	}
}

// BooleanEvaluation resolves a boolean flag.
func (p *QuonfigProvider) BooleanEvaluation(
	_ context.Context,
	flag string,
	defaultValue bool,
	flatCtx openfeature.FlattenedContext,
) openfeature.BoolResolutionDetail {
	client := p.getClient()
	if client == nil {
		return openfeature.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewProviderNotReadyResolutionError("provider not initialized"),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	val, found, err := client.GetBoolValue(flag, qCtx)
	if err != nil || !found {
		resErr := toResolutionError(err, found)
		reason := defaultOrErrorReason(err, found)
		return openfeature.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: resErr,
				Reason:          reason,
			},
		}
	}
	return openfeature.BoolResolutionDetail{
		Value: val,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			Reason: openfeature.StaticReason,
		},
	}
}

// StringEvaluation resolves a string flag.
func (p *QuonfigProvider) StringEvaluation(
	_ context.Context,
	flag string,
	defaultValue string,
	flatCtx openfeature.FlattenedContext,
) openfeature.StringResolutionDetail {
	client := p.getClient()
	if client == nil {
		return openfeature.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewProviderNotReadyResolutionError("provider not initialized"),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	val, found, err := client.GetStringValue(flag, qCtx)
	if err != nil || !found {
		resErr := toResolutionError(err, found)
		reason := defaultOrErrorReason(err, found)
		return openfeature.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: resErr,
				Reason:          reason,
			},
		}
	}
	return openfeature.StringResolutionDetail{
		Value: val,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			Reason: openfeature.StaticReason,
		},
	}
}

// FloatEvaluation resolves a float64 flag.
func (p *QuonfigProvider) FloatEvaluation(
	_ context.Context,
	flag string,
	defaultValue float64,
	flatCtx openfeature.FlattenedContext,
) openfeature.FloatResolutionDetail {
	client := p.getClient()
	if client == nil {
		return openfeature.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewProviderNotReadyResolutionError("provider not initialized"),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	val, found, err := client.GetFloatValue(flag, qCtx)
	if err != nil || !found {
		resErr := toResolutionError(err, found)
		reason := defaultOrErrorReason(err, found)
		return openfeature.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: resErr,
				Reason:          reason,
			},
		}
	}
	return openfeature.FloatResolutionDetail{
		Value: val,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			Reason: openfeature.StaticReason,
		},
	}
}

// IntEvaluation resolves an int64 flag.
func (p *QuonfigProvider) IntEvaluation(
	_ context.Context,
	flag string,
	defaultValue int64,
	flatCtx openfeature.FlattenedContext,
) openfeature.IntResolutionDetail {
	client := p.getClient()
	if client == nil {
		return openfeature.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewProviderNotReadyResolutionError("provider not initialized"),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	val, found, err := client.GetIntValue(flag, qCtx)
	if err != nil || !found {
		resErr := toResolutionError(err, found)
		reason := defaultOrErrorReason(err, found)
		return openfeature.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: resErr,
				Reason:          reason,
			},
		}
	}
	return openfeature.IntResolutionDetail{
		Value: val,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			Reason: openfeature.StaticReason,
		},
	}
}

// ObjectEvaluation resolves an object (string_list or JSON) flag.
// It tries string_list first, then JSON.
func (p *QuonfigProvider) ObjectEvaluation(
	_ context.Context,
	flag string,
	defaultValue any,
	flatCtx openfeature.FlattenedContext,
) openfeature.InterfaceResolutionDetail {
	client := p.getClient()
	if client == nil {
		return openfeature.InterfaceResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewProviderNotReadyResolutionError("provider not initialized"),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)

	// Try string_list first.
	listVal, found, err := client.GetStringSliceValue(flag, qCtx)
	if err == nil && found {
		// Convert []string to []any for OpenFeature compatibility.
		result := make([]any, len(listVal))
		for i, s := range listVal {
			result[i] = s
		}
		return openfeature.InterfaceResolutionDetail{
			Value: result,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				Reason: openfeature.StaticReason,
			},
		}
	}

	// Try JSON.
	jsonVal, found, err := client.GetJSONValue(flag, qCtx)
	if err == nil && found {
		return openfeature.InterfaceResolutionDetail{
			Value: jsonVal,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				Reason: openfeature.StaticReason,
			},
		}
	}

	// Not found or error — use default.
	resErr := toResolutionError(err, found)
	reason := defaultOrErrorReason(err, found)
	return openfeature.InterfaceResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			ResolutionError: resErr,
			Reason:          reason,
		},
	}
}

// GetClient returns the underlying Quonfig client for native-only features
// (Keys(), raw config access, duration values, log levels, etc.).
// Returns nil if the provider has not been initialized yet.
func (p *QuonfigProvider) GetClient() *quonfig.Client {
	return p.getClient()
}

// --- private helpers ---

func (p *QuonfigProvider) getClient() *quonfig.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}

func (p *QuonfigProvider) buildQuonfigOptions() []quonfig.Option {
	var opts []quonfig.Option

	if p.opts.SDKKey != "" {
		opts = append(opts, quonfig.WithAPIKey(p.opts.SDKKey))
	}
	if p.opts.DataDir != "" {
		opts = append(opts, quonfig.WithDataDir(p.opts.DataDir))
	}
	if p.opts.Environment != "" {
		opts = append(opts, quonfig.WithEnvironment(p.opts.Environment))
	}

	// Wire config change notifications -> OpenFeature PROVIDER_CONFIGURATION_CHANGED event.
	// Only emit after the provider is ready to avoid sending a spurious config-change
	// event during the initial load (before Init() has completed).
	opts = append(opts, quonfig.WithOnConfigUpdate(func() {
		p.mu.RLock()
		ready := p.ready
		p.mu.RUnlock()
		if ready {
			p.sendEvent(openfeature.ProviderConfigChange, openfeature.ProviderEventDetails{
				FlagChanges: []string{},
			})
		}
	}))

	opts = append(opts, p.opts.AdditionalOptions...)
	return opts
}

// defaultOrErrorReason returns DefaultReason when the flag was simply not found,
// and ErrorReason for all other error conditions.
func defaultOrErrorReason(err error, found bool) openfeature.Reason {
	if isFlagNotFound(err, found) {
		return openfeature.DefaultReason
	}
	if err != nil {
		return openfeature.ErrorReason
	}
	return openfeature.DefaultReason
}

func (p *QuonfigProvider) sendEvent(eventType openfeature.EventType, details openfeature.ProviderEventDetails) {
	event := openfeature.Event{
		ProviderName:         "quonfig",
		EventType:            eventType,
		ProviderEventDetails: details,
	}
	select {
	case p.eventCh <- event:
	default:
		// Channel full — drop the event rather than blocking.
	}
}
