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
	"encoding/json"
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
	opts    Options
	client  *quonfig.Client
	mu      sync.RWMutex
	ready   bool // set to true after Init() completes successfully
	eventCh chan openfeature.Event
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

// notReadyDetail returns the standard ProviderResolutionDetail used when the
// provider is invoked before Init has finished.
func notReadyDetail() openfeature.ProviderResolutionDetail {
	return openfeature.ProviderResolutionDetail{
		ResolutionError: openfeature.NewProviderNotReadyResolutionError("provider not initialized"),
		Reason:          openfeature.ErrorReason,
		Variant:         "default",
		FlagMetadata:    openfeature.FlagMetadata{},
	}
}

// buildResolutionDetail copies Variant and FlagMetadata from the SDK-side
// EvaluationDetails into an OpenFeature ProviderResolutionDetail. Caller fills
// in the actual coerced Value separately.
func buildResolutionDetail(details quonfig.EvaluationDetails) openfeature.ProviderResolutionDetail {
	md := openfeature.FlagMetadata{}
	for k, v := range details.FlagMetadata {
		md[k] = v
	}
	return openfeature.ProviderResolutionDetail{
		ResolutionError: resolutionErrorFor(details),
		Reason:          reasonFor(details),
		Variant:         details.Variant,
		FlagMetadata:    md,
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
		return openfeature.BoolResolutionDetail{Value: defaultValue, ProviderResolutionDetail: notReadyDetail()}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	details := client.EvaluateDetails(flag, qCtx)
	prd := buildResolutionDetail(details)
	if details.ErrorCode != quonfig.ErrorCodeNone || details.Value == nil {
		return openfeature.BoolResolutionDetail{Value: defaultValue, ProviderResolutionDetail: prd}
	}
	return openfeature.BoolResolutionDetail{Value: details.Value.BoolValue(), ProviderResolutionDetail: prd}
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
		return openfeature.StringResolutionDetail{Value: defaultValue, ProviderResolutionDetail: notReadyDetail()}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	details := client.EvaluateDetails(flag, qCtx)
	prd := buildResolutionDetail(details)
	if details.ErrorCode != quonfig.ErrorCodeNone || details.Value == nil {
		return openfeature.StringResolutionDetail{Value: defaultValue, ProviderResolutionDetail: prd}
	}
	return openfeature.StringResolutionDetail{Value: details.Value.StringValue(), ProviderResolutionDetail: prd}
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
		return openfeature.FloatResolutionDetail{Value: defaultValue, ProviderResolutionDetail: notReadyDetail()}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	details := client.EvaluateDetails(flag, qCtx)
	prd := buildResolutionDetail(details)
	if details.ErrorCode != quonfig.ErrorCodeNone || details.Value == nil {
		return openfeature.FloatResolutionDetail{Value: defaultValue, ProviderResolutionDetail: prd}
	}
	return openfeature.FloatResolutionDetail{Value: details.Value.DoubleValue(), ProviderResolutionDetail: prd}
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
		return openfeature.IntResolutionDetail{Value: defaultValue, ProviderResolutionDetail: notReadyDetail()}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	details := client.EvaluateDetails(flag, qCtx)
	prd := buildResolutionDetail(details)
	if details.ErrorCode != quonfig.ErrorCodeNone || details.Value == nil {
		return openfeature.IntResolutionDetail{Value: defaultValue, ProviderResolutionDetail: prd}
	}
	return openfeature.IntResolutionDetail{Value: details.Value.IntValue(), ProviderResolutionDetail: prd}
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
		return openfeature.InterfaceResolutionDetail{Value: defaultValue, ProviderResolutionDetail: notReadyDetail()}
	}

	qCtx := MapContext(flatCtx, p.opts.TargetingKeyMapping)
	details := client.EvaluateDetails(flag, qCtx)
	prd := buildResolutionDetail(details)
	if details.ErrorCode != quonfig.ErrorCodeNone || details.Value == nil {
		return openfeature.InterfaceResolutionDetail{Value: defaultValue, ProviderResolutionDetail: prd}
	}

	rawVal := details.Value

	// Try string_list first.
	if listVal := rawVal.StringListValue(); listVal != nil {
		result := make([]any, len(listVal))
		for i, s := range listVal {
			result[i] = s
		}
		return openfeature.InterfaceResolutionDetail{Value: result, ProviderResolutionDetail: prd}
	}

	// Try JSON.
	s := rawVal.StringValue()
	if s != "" {
		var jsonVal interface{}
		if jsonErr := json.Unmarshal([]byte(s), &jsonVal); jsonErr == nil {
			return openfeature.InterfaceResolutionDetail{Value: jsonVal, ProviderResolutionDetail: prd}
		}
	}

	// Return the raw string value as a fallback.
	return openfeature.InterfaceResolutionDetail{Value: rawVal.StringValue(), ProviderResolutionDetail: prd}
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

// evalReasonToOF maps a Quonfig EvalReason to an OpenFeature Reason string.
func evalReasonToOF(r quonfig.EvalReason) openfeature.Reason {
	switch r {
	case quonfig.ReasonStatic:
		return openfeature.StaticReason
	case quonfig.ReasonTargetingMatch:
		return openfeature.TargetingMatchReason
	case quonfig.ReasonSplit:
		return openfeature.SplitReason
	case quonfig.ReasonDefault:
		return openfeature.DefaultReason
	default:
		return openfeature.UnknownReason
	}
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
