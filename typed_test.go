package openfeaturego_test

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
)

// Typed-getter contract (qfg-zvyy) over the integration-test-data
// getter.contract.* fixtures: a value of another type is TYPE_MISMATCH plus
// the caller's default, never a coerced value.

func assertTypeMismatch(t *testing.T, err error, reason openfeature.Reason) {
	t.Helper()
	assert.ErrorContains(t, err, string(openfeature.TypeMismatchCode))
	assert.Equal(t, openfeature.ErrorReason, reason)
}

func TestTypedContract_WrongTypeReturnsDefaultAndTypeMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p := newDataDirProvider(t)
	bg := context.Background()

	// A stored string "false" is a string, not a bool.
	b := p.BooleanEvaluation(bg, "getter.contract.string.false", true, nil)
	assert.True(t, b.Value)
	assertTypeMismatch(t, b.Error(), b.Reason)

	s := p.StringEvaluation(bg, "getter.contract.int", "DEFAULT", nil)
	assert.Equal(t, "DEFAULT", s.Value)
	assertTypeMismatch(t, s.Error(), s.Reason)

	i := p.IntEvaluation(bg, "getter.contract.string.number", 7, nil)
	assert.Equal(t, int64(7), i.Value)
	assertTypeMismatch(t, i.Error(), i.Reason)

	o := p.ObjectEvaluation(bg, "getter.contract.string", "fallback", nil)
	assert.Equal(t, "fallback", o.Value)
	assertTypeMismatch(t, o.Error(), o.Reason)
}

func TestTypedContract_SafeNumberConversionsOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p := newDataDirProvider(t)
	bg := context.Background()

	f := p.FloatEvaluation(bg, "getter.contract.int", 7.5, nil)
	assert.NoError(t, f.Error())
	assert.Equal(t, 30.0, f.Value)

	whole := p.IntEvaluation(bg, "getter.contract.double.whole", 7, nil)
	assert.NoError(t, whole.Error())
	assert.Equal(t, int64(3), whole.Value)

	for _, key := range []string{"getter.contract.double", "getter.contract.double.huge", "getter.contract.double.unsafe"} {
		d := p.IntEvaluation(bg, key, 7, nil)
		assert.Equal(t, int64(7), d.Value, key)
		assertTypeMismatch(t, d.Error(), d.Reason)
	}
}

func TestTypedContract_MalformedNumberIsParseError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p := newDataDirProvider(t)
	d := p.IntEvaluation(context.Background(), "getter.contract.int.malformed", 7, nil)
	assert.Equal(t, int64(7), d.Value)
	assert.ErrorContains(t, d.Error(), string(openfeature.ParseErrorCode))
	assert.Equal(t, openfeature.ErrorReason, d.Reason)
}

func TestTypedContract_ObjectReturnsNativeJSONAndLists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p := newDataDirProvider(t)
	bg := context.Background()

	j := p.ObjectEvaluation(bg, "getter.contract.json", nil, nil)
	assert.NoError(t, j.Error())
	assert.Equal(t, map[string]any{"a": float64(1)}, j.Value)

	l := p.ObjectEvaluation(bg, "getter.contract.string_list", nil, nil)
	assert.NoError(t, l.Error())
	assert.Equal(t, []any{"a", "b"}, l.Value)
}

func TestTypedContract_DurationReadsAsISOString(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	p := newDataDirProvider(t)
	d := p.StringEvaluation(context.Background(), "getter.contract.duration", "DEFAULT", nil)
	assert.NoError(t, d.Error())
	assert.Equal(t, "PT30S", d.Value)
}
