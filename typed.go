package openfeaturego

import (
	"fmt"
	"math"

	"github.com/open-feature/go-sdk/openfeature"
	quonfig "github.com/quonfig/sdk-go/v2"
)

// The provider resolves through the untyped EvaluateDetails (for reason,
// variant and metadata), so it applies the sdk-go v2 typed-getter contract
// (qfg-zvyy) itself: a value of another type is TYPE_MISMATCH and the
// caller's default, never a coerced value. Strings are never coerced; the
// only conversions are int -> float and a whole float within +/-(2^53-1)
// -> int. A stored number or duration that did not parse is PARSE_ERROR.

const maxSafeInt = 1<<53 - 1

// typedFailure is the resolution detail for an evaluated value that cannot be
// returned as the requested type.
func typedFailure(details quonfig.EvaluationDetails, err openfeature.ResolutionError) openfeature.ProviderResolutionDetail {
	prd := buildResolutionDetail(details)
	prd.ResolutionError = err
	prd.Reason = openfeature.ErrorReason
	return prd
}

func mismatch(flag string, v *quonfig.Value, want quonfig.ValueType) openfeature.ResolutionError {
	return openfeature.NewTypeMismatchResolutionError(fmt.Sprintf("config %q has type %s, expected %s", flag, v.Type, want))
}

func malformed(flag string, want quonfig.ValueType) openfeature.ResolutionError {
	return openfeature.NewParseErrorResolutionError(fmt.Sprintf("config %q has a malformed %s value", flag, want))
}

func boolOf(flag string, v *quonfig.Value) (bool, *openfeature.ResolutionError) {
	if b, ok := v.Value.(bool); ok && v.Type == quonfig.ValueTypeBool {
		return b, nil
	}
	err := mismatch(flag, v, quonfig.ValueTypeBool)
	return false, &err
}

// stringOf also reads a duration config as its ISO 8601 string: OpenFeature
// has no duration type, so StringValue is the documented way to read one. A
// stored duration that is not valid ISO 8601 (e.g. "30s") is PARSE_ERROR.
func stringOf(flag string, v *quonfig.Value) (string, *openfeature.ResolutionError) {
	switch v.Type {
	case quonfig.ValueTypeString, quonfig.ValueTypeLogLevel, quonfig.ValueTypeDuration:
	default:
		err := mismatch(flag, v, quonfig.ValueTypeString)
		return "", &err
	}
	if s, ok := v.Value.(string); ok {
		if v.Type == quonfig.ValueTypeDuration {
			if _, parseErr := quonfig.ParseISO8601Duration(s); parseErr != nil {
				err := malformed(flag, quonfig.ValueTypeDuration)
				return "", &err
			}
		}
		return s, nil
	}
	err := mismatch(flag, v, quonfig.ValueTypeString)
	return "", &err
}

func floatOf(flag string, v *quonfig.Value) (float64, *openfeature.ResolutionError) {
	switch v.Type {
	case quonfig.ValueTypeDouble:
		if f, ok := v.Value.(float64); ok {
			return f, nil
		}
		err := malformed(flag, quonfig.ValueTypeDouble)
		return 0, &err
	case quonfig.ValueTypeInt:
		i, err := intOf(flag, v)
		return float64(i), err
	}
	err := mismatch(flag, v, quonfig.ValueTypeDouble)
	return 0, &err
}

func intOf(flag string, v *quonfig.Value) (int64, *openfeature.ResolutionError) {
	switch v.Type {
	case quonfig.ValueTypeInt:
		if i, ok := v.Value.(int64); ok {
			return i, nil
		}
		err := malformed(flag, quonfig.ValueTypeInt)
		return 0, &err
	case quonfig.ValueTypeDouble:
		f, err := floatOf(flag, v)
		if err != nil {
			return 0, err
		}
		if f == math.Trunc(f) && math.Abs(f) <= maxSafeInt {
			return int64(f), nil
		}
	}
	err := mismatch(flag, v, quonfig.ValueTypeInt)
	return 0, &err
}

// objectOf reads a string_list (as []any) or a json config (its native value).
func objectOf(flag string, v *quonfig.Value) (any, *openfeature.ResolutionError) {
	switch v.Type {
	case quonfig.ValueTypeStringList:
		list := v.StringListValue()
		result := make([]any, len(list))
		for i, s := range list {
			result[i] = s
		}
		return result, nil
	case quonfig.ValueTypeJSON:
		return v.Value, nil
	}
	err := mismatch(flag, v, quonfig.ValueTypeJSON)
	return nil, &err
}
