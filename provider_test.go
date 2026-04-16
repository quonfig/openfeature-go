package openfeaturego_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	openfeaturego "github.com/quonfig/openfeature-go"
	quonfig "github.com/quonfig/sdk-go"
)

// integrationTestDataDir returns the absolute path to the integration-test-data fixtures.
func integrationTestDataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "integration-test-data", "data", "integration-tests")
}

// --- MapContext unit tests ---

func TestMapContext_DotNotation(t *testing.T) {
	ctx := map[string]any{
		"user.email": "alice@co.com",
	}
	qCtx := openfeaturego.MapContext(ctx, "user.id")
	require.NotNil(t, qCtx)
	// We can't directly inspect the ContextSet internals, but we can verify it's non-nil.
	// The actual correctness is validated via integration tests.
}

func TestMapContext_EmptyNamespace(t *testing.T) {
	// "country" has no dot -> goes to empty-string namespace.
	ctx := map[string]any{
		"country": "US",
	}
	qCtx := openfeaturego.MapContext(ctx, "user.id")
	require.NotNil(t, qCtx)
}

func TestMapContext_TargetingKey(t *testing.T) {
	ctx := map[string]any{
		"targetingKey": "user-123",
	}
	qCtx := openfeaturego.MapContext(ctx, "user.id")
	require.NotNil(t, qCtx)
}

func TestMapContext_TargetingKeyCustomMapping(t *testing.T) {
	ctx := map[string]any{
		"targetingKey": "org-456",
	}
	qCtx := openfeaturego.MapContext(ctx, "org.key")
	require.NotNil(t, qCtx)
}

func TestMapContext_MultiDotSplitOnFirst(t *testing.T) {
	// "user.ip.address" -> namespace "user", key "ip.address"
	ctx := map[string]any{
		"user.ip.address": "1.2.3.4",
	}
	qCtx := openfeaturego.MapContext(ctx, "user.id")
	require.NotNil(t, qCtx)
}

func TestMapContext_Empty(t *testing.T) {
	qCtx := openfeaturego.MapContext(map[string]any{}, "user.id")
	assert.Nil(t, qCtx)
}

func TestMapContext_Nil(t *testing.T) {
	qCtx := openfeaturego.MapContext(nil, "user.id")
	assert.Nil(t, qCtx)
}

func TestMapContext_NilValues(t *testing.T) {
	ctx := map[string]any{
		"user.email": nil,
	}
	qCtx := openfeaturego.MapContext(ctx, "user.id")
	// nil values are skipped
	assert.Nil(t, qCtx)
}

func TestMapContext_TargetingKeyNoMapping(t *testing.T) {
	// targetingKeyMapping without a dot -> empty namespace, prop = mapping
	ctx := map[string]any{
		"targetingKey": "abc",
	}
	qCtx := openfeaturego.MapContext(ctx, "userid")
	require.NotNil(t, qCtx)
}

// --- Provider unit tests (not-initialized path) ---

func TestProvider_NotInitialized_ReturnsDefault(t *testing.T) {
	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
		DataDir:     "/nonexistent/path",
		Environment: "Production",
	})
	// Do NOT call Init() — provider is not initialized.

	bg := context.Background()
	detail := provider.BooleanEvaluation(bg, "some-flag", true, nil)
	assert.True(t, detail.Value) // returns default
	assert.Equal(t, openfeature.ErrorReason, detail.Reason)

	sDetail := provider.StringEvaluation(bg, "some-flag", "default", nil)
	assert.Equal(t, "default", sDetail.Value)

	fDetail := provider.FloatEvaluation(bg, "some-flag", 3.14, nil)
	assert.InDelta(t, 3.14, fDetail.Value, 0.001)

	iDetail := provider.IntEvaluation(bg, "some-flag", int64(42), nil)
	assert.Equal(t, int64(42), iDetail.Value)

	oDetail := provider.ObjectEvaluation(bg, "some-flag", "fallback", nil)
	assert.Equal(t, "fallback", oDetail.Value)
}

func TestProvider_Metadata(t *testing.T) {
	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{})
	assert.Equal(t, "quonfig", provider.Metadata().Name)
}

func TestProvider_Hooks(t *testing.T) {
	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{})
	assert.Nil(t, provider.Hooks())
}

func TestProvider_DefaultTargetingKeyMapping(t *testing.T) {
	// Options with no TargetingKeyMapping should default to "user.id".
	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{})
	// We can't inspect the field directly, but we exercise the code path.
	assert.NotNil(t, provider)
}

// --- Integration tests (require integration-test-data) ---

func newDataDirProvider(t *testing.T) *openfeaturego.QuonfigProvider {
	t.Helper()
	dataDir := integrationTestDataDir()
	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
		DataDir:     dataDir,
		Environment: "Production",
		AdditionalOptions: []quonfig.Option{
			quonfig.WithAllTelemetryDisabled(),
		},
	})
	err := provider.Init(openfeature.EvaluationContext{})
	require.NoError(t, err)
	t.Cleanup(provider.Shutdown)
	return provider
}

func TestIntegration_BooleanFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// always.true should be true.
	detail := provider.BooleanEvaluation(bg, "always.true", false, nil)
	assert.NoError(t, detail.Error())
	assert.True(t, detail.Value)
	assert.Equal(t, openfeature.StaticReason, detail.Reason)
}

func TestIntegration_StringFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// brand.new.string has value "hello.world" in Production.
	detail := provider.StringEvaluation(bg, "brand.new.string", "default", nil)
	assert.NoError(t, detail.Error())
	assert.Equal(t, "hello.world", detail.Value)
	assert.Equal(t, openfeature.StaticReason, detail.Reason)
}

func TestIntegration_IntFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// brand.new.int has value 123 in Production.
	detail := provider.IntEvaluation(bg, "brand.new.int", 0, nil)
	assert.NoError(t, detail.Error())
	assert.Equal(t, int64(123), detail.Value)
}

func TestIntegration_UnknownFlag_ReturnsDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// An unknown flag should return the default value with DefaultReason and a non-empty error.
	detail := provider.BooleanEvaluation(bg, "this-flag-does-not-exist", true, nil)
	assert.True(t, detail.Value) // returns default
	assert.Equal(t, openfeature.DefaultReason, detail.Reason)
	assert.NotEmpty(t, detail.ResolutionError.Error(), "expected resolution error for missing flag")
}

func TestIntegration_UnknownFlag_ErrorCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	detail := provider.StringEvaluation(bg, "nonexistent-flag-xyz", "fallback", nil)
	assert.Equal(t, "fallback", detail.Value)
	assert.Equal(t, openfeature.DefaultReason, detail.Reason)
	// Error should indicate FLAG_NOT_FOUND
	assert.NotEmpty(t, detail.ResolutionError.Error())
}

func TestIntegration_DotNotationContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// my-test-key: default "my-test-value" in Production.
	// With namespace.key = "present" -> "namespace-value".
	noCtxDetail := provider.StringEvaluation(bg, "my-test-key", "default", nil)
	assert.Equal(t, "my-test-value", noCtxDetail.Value)

	withCtxDetail := provider.StringEvaluation(bg, "my-test-key", "default", openfeature.FlattenedContext{
		"namespace.key": "present",
	})
	assert.Equal(t, "namespace-value", withCtxDetail.Value)
}

func TestIntegration_TargetingKeyContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// feature-flag.in-segment.positive: true when user.key is in the "users" segment.
	// Pass targetingKey -> maps to "user.id" by default (but here user.key is checked).
	// Use dot-notation for user.key directly.
	ctxIn := openfeature.FlattenedContext{
		"user.key": "jeffrey",
	}
	detail := provider.BooleanEvaluation(bg, "feature-flag.in-segment.positive", false, ctxIn)
	assert.True(t, detail.Value)

	ctxOut := openfeature.FlattenedContext{
		"user.key": "unknown-user",
	}
	detailOut := provider.BooleanEvaluation(bg, "feature-flag.in-segment.positive", true, ctxOut)
	assert.False(t, detailOut.Value)
}

func TestIntegration_EventChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dataDir := integrationTestDataDir()
	provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
		DataDir:     dataDir,
		Environment: "Production",
	})

	eventCh := provider.EventChannel()
	require.NotNil(t, eventCh)

	err := provider.Init(openfeature.EvaluationContext{})
	require.NoError(t, err)
	defer provider.Shutdown()

	// Should receive PROVIDER_READY after Init.
	select {
	case evt := <-eventCh:
		assert.Equal(t, openfeature.ProviderReady, evt.EventType)
	default:
		t.Error("expected PROVIDER_READY event on channel")
	}
}

func TestIntegration_GetClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	client := provider.GetClient()
	require.NotNil(t, client)
	keys := client.Keys()
	assert.Greater(t, len(keys), 0, "expected at least one key from native client")
}

// --- Reason field integration tests ---

func TestIntegration_StaticReason_NoTargetingRules(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// brand.new.string has no targeting rules -> STATIC
	detail := provider.StringEvaluation(bg, "brand.new.string", "default", nil)
	assert.NoError(t, detail.Error())
	assert.Equal(t, "hello.world", detail.Value)
	assert.Equal(t, openfeature.StaticReason, detail.Reason)
}

func TestIntegration_TargetingMatchReason_RuleMatches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// of.targeting: returns true when user.plan is "pro" (TARGETING_MATCH)
	detail := provider.BooleanEvaluation(bg, "of.targeting", false, openfeature.FlattenedContext{
		"user.plan": "pro",
	})
	assert.NoError(t, detail.Error())
	assert.True(t, detail.Value)
	assert.Equal(t, openfeature.TargetingMatchReason, detail.Reason)
}

func TestIntegration_TargetingMatchReason_Fallthrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// of.targeting without user.plan="pro" -> falls through to ALWAYS_TRUE rule (TARGETING_MATCH)
	detail := provider.BooleanEvaluation(bg, "of.targeting", true, nil)
	assert.NoError(t, detail.Error())
	assert.False(t, detail.Value)
	assert.Equal(t, openfeature.TargetingMatchReason, detail.Reason)
}

func TestIntegration_SplitReason_WeightedValue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// of.weighted uses a weighted_values rule -> SPLIT
	// The targetingKey maps to user.id by default for bucketing
	detail := provider.StringEvaluation(bg, "of.weighted", "default", openfeature.FlattenedContext{
		"targetingKey": "92a202f2",
	})
	assert.NoError(t, detail.Error())
	assert.NotEqual(t, "default", detail.Value)
	assert.Equal(t, openfeature.SplitReason, detail.Reason)
}
