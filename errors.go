package openfeaturego

import (
	"github.com/open-feature/go-sdk/openfeature"
	quonfig "github.com/quonfig/sdk-go/v2"
)

// sdkErrorCodeParseError is the SDK's PARSE_ERROR code, matched by its string
// value (qfg-wg1n). Newer sdk-go releases report malformed data with it; the
// sdk-go v2.0.0 this module requires has no exported constant for it, so the
// provider does not depend on one and keeps building against that floor.
const sdkErrorCodeParseError quonfig.ErrorCode = "PARSE_ERROR"

// resolutionErrorFor maps a Quonfig EvaluationDetails to an OpenFeature
// ResolutionError. The mapping reads the typed ErrorCode set by the SDK at the
// actual error site -- no inference from error message text. This is a
// deliberate change from the prior string-matching approach: any tweak to an
// SDK error message is now decoupled from the OpenFeature error mapping.
func resolutionErrorFor(details quonfig.EvaluationDetails) openfeature.ResolutionError {
	msg := details.ErrorMessage
	switch details.ErrorCode {
	case quonfig.ErrorCodeNone:
		return openfeature.ResolutionError{}
	case quonfig.ErrorCodeFlagNotFound:
		return openfeature.NewFlagNotFoundResolutionError(msg)
	case quonfig.ErrorCodeTypeMismatch:
		// The provider resolves through the untyped EvaluateDetails, where
		// sdk-go raises TYPE_MISMATCH only for ErrUnableToCoerce: an ENV_VAR-
		// provided value (e.g. "twelve" for an int config) that does not parse
		// as the config's own type. That is malformed data, not a caller
		// reading the wrong type, so it is PARSE_ERROR (qfg-9xsl). Wrong-type
		// reads are detected by the provider itself (typed.go mismatch).
		return openfeature.NewParseErrorResolutionError(msg)
	case sdkErrorCodeParseError:
		// A newer sdk-go names malformed data PARSE_ERROR itself: pass it
		// straight through.
		return openfeature.NewParseErrorResolutionError(msg)
	case quonfig.ErrorCodeProviderNotReady:
		return openfeature.NewProviderNotReadyResolutionError(msg)
	default:
		return openfeature.NewGeneralResolutionError(msg)
	}
}

// reasonFor maps a Quonfig EvaluationDetails to an OpenFeature Reason.
// On error, FLAG_NOT_FOUND surfaces as DefaultReason (the spec lets providers
// pick either DEFAULT or ERROR for missing flags; we pick DEFAULT to match the
// existing provider contract); other error codes surface as ErrorReason.
// On success, the reason is mapped from the SDK's EvalReason.
func reasonFor(details quonfig.EvaluationDetails) openfeature.Reason {
	if details.ErrorCode == quonfig.ErrorCodeFlagNotFound {
		return openfeature.DefaultReason
	}
	if details.ErrorCode != quonfig.ErrorCodeNone {
		return openfeature.ErrorReason
	}
	return evalReasonToOF(details.Reason)
}
