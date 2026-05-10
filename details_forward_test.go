package openfeaturego_test

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	openfeaturego "github.com/quonfig/openfeature-go"
	quonfig "github.com/quonfig/sdk-go"
)

// TestDetailsForward_StaticVariantAndMetadata verifies that the provider
// forwards Variant and FlagMetadata from the SDK's EvaluationDetails into
// ProviderResolutionDetail (qfg-zbz7 acceptance: round-trip metadata).
func TestDetailsForward_StaticVariantAndMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	// brand.new.string is a STATIC config (no targeting rules).
	detail := provider.StringEvaluation(bg, "brand.new.string", "default", nil)
	require.NoError(t, detail.Error())
	assert.Equal(t, "static", detail.Variant)
	assert.Equal(t, openfeature.StaticReason, detail.Reason)
	// flagMetadata round-trip: configId, configType, environment present.
	assert.NotEmpty(t, detail.FlagMetadata["configId"], "configId must be present in FlagMetadata")
	assert.Equal(t, "CONFIG", detail.FlagMetadata["configType"])
	assert.Equal(t, "Production", detail.FlagMetadata["environment"])
	// ruleIndex / weightedValueIndex must be omitted for STATIC.
	_, hasRule := detail.FlagMetadata["ruleIndex"]
	_, hasWVI := detail.FlagMetadata["weightedValueIndex"]
	assert.False(t, hasRule, "ruleIndex must be omitted for STATIC")
	assert.False(t, hasWVI, "weightedValueIndex must be omitted for STATIC")
}

// TestDetailsForward_TargetingMatchVariantAndMetadata covers the targeting-match
// path: variant must be "targeting:0" and ruleIndex must be 0.
func TestDetailsForward_TargetingMatchVariantAndMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	detail := provider.BooleanEvaluation(bg, "of.targeting", false, openfeature.FlattenedContext{
		"user.plan": "pro",
	})
	require.NoError(t, detail.Error())
	assert.True(t, detail.Value)
	assert.Equal(t, "targeting:0", detail.Variant)
	assert.Equal(t, openfeature.TargetingMatchReason, detail.Reason)
	idx, err := detail.FlagMetadata.GetInt("ruleIndex")
	require.NoError(t, err)
	assert.Equal(t, int64(0), idx)
}

// TestDetailsForward_SplitVariantAndMetadata covers the split path:
// variant must be "split:N" and weightedValueIndex must be N.
func TestDetailsForward_SplitVariantAndMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	provider := newDataDirProvider(t)
	bg := context.Background()

	detail := provider.StringEvaluation(bg, "of.weighted", "default", openfeature.FlattenedContext{
		"targetingKey": "92a202f2",
	})
	require.NoError(t, detail.Error())
	assert.Equal(t, openfeature.SplitReason, detail.Reason)
	wvi, err := detail.FlagMetadata.GetInt("weightedValueIndex")
	require.NoError(t, err)
	assert.Contains(t, []string{"split:0", "split:1"}, detail.Variant)
	// The variant index must agree with the weightedValueIndex metadata.
	if wvi == 0 {
		assert.Equal(t, "split:0", detail.Variant)
	} else if wvi == 1 {
		assert.Equal(t, "split:1", detail.Variant)
	}
	// rule index is also present for SPLIT.
	rIdx, err := detail.FlagMetadata.GetInt("ruleIndex")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, rIdx, int64(0))
}

// TestErrorMapping_TypedErrorCodeIgnoresMessageText is the regression test that
// proves we removed the brittle string-substring inference from openfeature-go's
// errors.go. We synthesize an EvaluationDetails with an intentionally weird
// ErrorMessage that does NOT contain any of the old trigger words ("not found",
// "type", "coerce") and verify the typed ErrorCode still produces the right
// ResolutionError category.
func TestErrorMapping_TypedErrorCodeIgnoresMessageText(t *testing.T) {
	cases := []struct {
		name     string
		details  quonfig.EvaluationDetails
		wantCode openfeature.ErrorCode
	}{
		{
			name: "FlagNotFound with message that has no 'not found' substring",
			details: quonfig.EvaluationDetails{
				ErrorCode:    quonfig.ErrorCodeFlagNotFound,
				ErrorMessage: "🦄 the unicorn ate the config", // deliberately weird
			},
			wantCode: openfeature.FlagNotFoundCode,
		},
		{
			name: "TypeMismatch with message that has no 'type' or 'coerce' substring",
			details: quonfig.EvaluationDetails{
				ErrorCode:    quonfig.ErrorCodeTypeMismatch,
				ErrorMessage: "🦄 unable to make number out of cheese",
			},
			wantCode: openfeature.TypeMismatchCode,
		},
		{
			name: "ProviderNotReady with arbitrary message",
			details: quonfig.EvaluationDetails{
				ErrorCode:    quonfig.ErrorCodeProviderNotReady,
				ErrorMessage: "still warming up the lasers",
			},
			wantCode: openfeature.ProviderNotReadyCode,
		},
		{
			name: "General with arbitrary message",
			details: quonfig.EvaluationDetails{
				ErrorCode:    quonfig.ErrorCodeGeneral,
				ErrorMessage: "something went sideways",
			},
			wantCode: openfeature.GeneralCode,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resErr := openfeaturego.ResolutionErrorFor(tc.details)
			// ResolutionError carries an opaque code; assert via Error()
			// containing the canonical OpenFeature code prefix.
			assert.Contains(t, resErr.Error(), string(tc.wantCode), "resolution error must surface the typed code regardless of message text")
		})
	}
}

// TestErrorMapping_FlagNotFoundReason reflects the existing OF-provider contract
// of reporting reason=DEFAULT (not ERROR) when a flag is missing. After moving
// to typed ErrorCode, that contract still holds.
func TestErrorMapping_FlagNotFoundReason(t *testing.T) {
	r := openfeaturego.ReasonFor(quonfig.EvaluationDetails{
		ErrorCode:    quonfig.ErrorCodeFlagNotFound,
		ErrorMessage: "this can be anything",
	})
	assert.Equal(t, openfeature.DefaultReason, r)
}

// TestErrorMapping_OtherErrorReason verifies non-not-found errors get
// reason=ERROR.
func TestErrorMapping_OtherErrorReason(t *testing.T) {
	r := openfeaturego.ReasonFor(quonfig.EvaluationDetails{
		ErrorCode:    quonfig.ErrorCodeTypeMismatch,
		ErrorMessage: "doesn't matter",
	})
	assert.Equal(t, openfeature.ErrorReason, r)
}
