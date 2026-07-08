# Changelog

## 1.2.0 - 2026-07-08

- Bump the `github.com/quonfig/sdk-go` pin from v1.1.0 to v1.2.0 to inherit its
  failover-observability work: a WARN at init when an explicit `WithAPIURLs`
  silently disables secondary failover, and a per-flush-window `failover`
  telemetry event carrying hedge-fired / guard-rejected / resolved-from counters
  (qfg-41nh). Both are additive and pull in no new dependencies. No change to
  this provider's own public API — the behavior rides in via the SDK bump.
  Coordinated 1.2.0 version stamp across the Quonfig SDK family.

## 1.1.0 - 2026-07-01

- Bump the `github.com/quonfig/sdk-go` pin from v1.0.0 to v1.1.0 to inherit the
  secondary-delivery failover work: request hedging across primary/secondary
  api-delivery, the reject-older generation guard, and the `gen<=0` carve-out
  that unblocks clients talking to pre-watermark servers (qfg-7h5d). No change
  to this provider's own public API — the failover behavior rides in via the
  SDK bump. Coordinated 1.1.0 version stamp across the Quonfig SDK family.

## 1.0.0 - 2026-06-06

- **Stable 1.0.0 release.** The Quonfig OpenFeature provider for Go is now declared
  stable and tracks `github.com/quonfig/sdk-go` v1.0.0. No API or behavior changes
  from 0.0.9 — this is a coordinated 1.0.0 version stamp across the entire Quonfig
  SDK family.

## 0.0.9 - 2026-06-02

- Bump the `github.com/quonfig/sdk-go` pin from v0.0.28 to v0.0.29 to inherit
  dev-context injection default-on (qfg-bw7g.9, via qfg-bw7g.3). No change to
  this provider's behavior — dev-context lives below the OpenFeature layer, so
  OpenFeature users now get `quonfig-user.email` injection by default in local
  dev (gated on the `qfg login` token file; inert in production).

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
