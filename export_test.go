package openfeaturego

// Test-only exports of unexported helpers so external (_test) packages can
// drive the typed-ErrorCode mapping path directly. These are not part of the
// public API.

var (
	ResolutionErrorFor = resolutionErrorFor
	ReasonFor          = reasonFor
)
