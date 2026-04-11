package openfeaturego

import (
	"errors"
	"strings"

	"github.com/open-feature/go-sdk/openfeature"
	quonfig "github.com/quonfig/sdk-go"
)

// isFlagNotFound returns true if the error represents a missing flag.
// This covers both the found=false case and explicit ErrNotFound errors.
func isFlagNotFound(err error, found bool) bool {
	if !found && err == nil {
		return true
	}
	if err != nil && errors.Is(err, quonfig.ErrNotFound) {
		return true
	}
	return false
}

// toResolutionError converts a native SDK error and found flag to an OpenFeature ResolutionError.
// If err is nil and found is false, returns FLAG_NOT_FOUND.
// If err is nil and found is true, returns a zero ResolutionError (no error).
func toResolutionError(err error, found bool) openfeature.ResolutionError {
	if err == nil && !found {
		return openfeature.NewFlagNotFoundResolutionError("flag not found")
	}
	if err == nil {
		return openfeature.ResolutionError{}
	}

	// ErrNotFound from the native SDK maps to FLAG_NOT_FOUND.
	if errors.Is(err, quonfig.ErrNotFound) {
		return openfeature.NewFlagNotFoundResolutionError(err.Error())
	}

	msg := strings.ToLower(err.Error())

	if strings.Contains(msg, "not found") || strings.Contains(msg, "no value found") {
		return openfeature.NewFlagNotFoundResolutionError(err.Error())
	}
	if strings.Contains(msg, "type") || strings.Contains(msg, "coerce") {
		return openfeature.NewTypeMismatchResolutionError(err.Error())
	}
	if strings.Contains(msg, "not initialized") || strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "initialization_timeout") {
		return openfeature.NewProviderNotReadyResolutionError(err.Error())
	}
	return openfeature.NewGeneralResolutionError(err.Error())
}
