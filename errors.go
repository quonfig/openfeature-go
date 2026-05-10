package openfeaturego

import (
	"github.com/open-feature/go-sdk/openfeature"
	quonfig "github.com/quonfig/sdk-go"
)

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
		return openfeature.NewTypeMismatchResolutionError(msg)
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
