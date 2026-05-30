# Changelog

## 0.0.8 - 2026-05-30

- Bump the `github.com/quonfig/sdk-go` pin from v0.0.26 to v0.0.28 and adapt to
  its `WithAPIKey` → `WithSdkKey` rename (qfg-ujcq). No change to this
  provider's own public API.

## 0.0.7 - 2026-05-28

- Bump the `github.com/quonfig/sdk-go` pin from v0.0.25 to v0.0.26 (sdk-1.0-unification).

## 0.0.6 - 2026-05-21

- Bump the `github.com/quonfig/sdk-go` pin from v0.0.23 to v0.0.25 (qfg-mie7).

## 0.0.5 - 2026-05-14

- Consume sdk-go's typed `ErrorCode` and forward `Variant` / `FlagMetadata` through the OpenFeature provider's evaluation details (qfg-zbz7).
- Bump the `github.com/quonfig/sdk-go` pin from v0.0.19 to v0.0.23. v0.0.19 predated the typed `ErrorCode` / `EvaluationDetails` / `EvaluateDetails` API the provider now depends on, which left CI red on main; the bump restores a green build.
