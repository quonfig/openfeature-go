# Changelog

## 1.6.0 - 2026-10-23

Requires sdk-go 1.10.0 (size caps, `callback_errors_total`). This release bumps
the `github.com/quonfig/sdk-go` pin from v1.8.0 through v1.9.0 to v1.10.0 and
maps sdk-go's startup-failure policy to OpenFeature provider status
(qfg-9p4i.12). The `go` directive stays at 1.25.0.

### Upgrade notes

- **UPGRADE: `Init` can now fail, so `SetProviderAndWait` can return an error
  and leave the provider in `ERROR` when delivery is down at startup.** This
  follows sdk-go 1.9.0, whose `NewClient` now blocks for the first config and
  fails by default. It ships as a minor under Jeff's Go semver override
  (project/plans/2026-10-05-init-failure-design.md, Decision 3, which also
  covers the openfeature-go pin bump; see sdk-go 1.9.0). Pass
  `quonfig.WithOnInitFailure(quonfig.InitFailureContinue)` to keep booting
  without config.
- From sdk-go 1.10.0: config responses over 16 MiB and SSE events over 4 MiB
  are rejected as failed fetches/stream attempts (qfg-6vno.20.7), and a
  recovered panic in a user callback counts in the internal
  `quonfig_sdk_callback_errors_total` counter instead of as a worker restart
  (qfg-6vno.26). Real workspaces are far below the caps. The provider's API is
  unchanged.

### Changed

- **`Init` now waits for the first config and can fail.** sdk-go 1.9.0's
  `NewClient` blocks until the first config arrives and, under the default
  `onInitFailure=fail`, returns `*quonfig.InitError` after `initTimeout`
  (10s) or at once when every delivery leg rejects the SDK key. `Init` returns
  that error, so `SetProviderAndWait` returns it and the provider status is
  `ERROR`. Before, `Init` always succeeded and the provider reported `READY`
  while delivery was down. sdk-go returns no client under `fail`, so the
  provider does not retry on its own; register it again or restart.

### Added

- `onInitFailure=continue` (`quonfig.WithOnInitFailure(quonfig.InitFailureContinue)`
  in `AdditionalOptions`): when the client starts without config, `Init`
  returns `nil` and the provider then emits `PROVIDER_ERROR` (status `ERROR`,
  evaluations return the default with `PROVIDER_NOT_READY`). When the client
  heals, the provider emits `PROVIDER_READY` instead of
  `PROVIDER_CONFIGURATION_CHANGED`.
- `onInitFailure=wait` keeps the status `NOT_READY` until the first config.
- README "Startup failure" section with the mode-to-status table.

## 1.5.1 - 2026-10-20

Restore Go 1.25 support. v1.4.0 and v1.5.0 required Go 1.26 by accident: a
grouped Dependabot bump moved `github.com/open-feature/go-sdk` from v1.17.2 to
v1.19.0, and go-sdk v1.19.0 declares `go 1.26.0`, which raised this module's
`go` directive to match. This release pins go-sdk to v1.18.0, the newest
release that supports Go 1.25, and sets the `go` directive back to 1.25.0.
The provider's API and behavior are unchanged; it used nothing that go-sdk
v1.19.0 added.

### Fixed

- **The module builds on Go 1.25 again.** The `go` directive is back to
  1.25.0 (go-sdk v1.18.0's own minimum). If you upgraded to Go 1.26 only for
  this provider, you can stay there; nothing else changes.

### Internal

- CI now also builds and tests with Go 1.25 and `GOTOOLCHAIN=local`, so a
  dependency that raises the minimum Go version fails the PR.
- Dependabot no longer folds `github.com/open-feature/go-sdk` into the grouped
  minor/patch PR. Its bumps arrive as their own PRs so a `go` directive change
  is visible in review.

## 1.5.0 - 2026-10-20

Bump the `github.com/quonfig/sdk-go` pin from v1.7.0 to v1.8.0. This
provider's own API and OpenFeature events are unchanged. sdk-go 1.8.0 changes
when the underlying client counts its SSE stream as connected, and fixes a
deadlock in the config-update callback.

### Upgrade notes

- **The stream counts as connected at its first frame (qfg-goi1.10.9,
  qfg-goi1.10.39).** If you read connection state from the underlying client
  (`provider.GetClient().ConnectionState()`, `FallbackPollerActive()`, or a
  `quonfig.WithSSEStateCallback` passed in `AdditionalOptions`), the client now
  reports `connected` only after the stream delivers its first config envelope
  or keepalive, not when the 200 response headers arrive. Against Quonfig's
  servers these arrive together, so you should see no difference. Behind a
  proxy that sends headers early, `connected` can come up to one keepalive
  interval (30 s) later. A stream that closes before any frame now counts as a
  failed attempt: reconnects back off and the fallback poller takes over,
  where before the client could reconnect in a tight loop and keep stale
  config. `PROVIDER_READY` timing is unchanged; it still follows the initial
  config load.

### Fixed (inherited)

- **The config-update callback no longer deadlocks when it calls the client
  (qfg-eefx).** sdk-go ran `OnConfigUpdate` while holding its install lock, so
  a callback that called `Refresh()` blocked forever and could stop SSE and
  polling from starting. The callback now runs after the lock is released.
  This affects you only if you pass your own `quonfig.WithOnConfigUpdate` in
  `AdditionalOptions`; the provider's built-in callback never calls back into
  the client. `PROVIDER_CONFIGURATION_CHANGED` events are still emitted once
  per config install, in order.

## 1.4.0 - 2026-10-16

Bump the `github.com/quonfig/sdk-go` pin from v1.6.0 to v1.7.0. This
provider's own API is unchanged, but sdk-go 1.7.0 changes evaluation and
connection behavior that OpenFeature users see through `ObjectValue`,
`BooleanValue`, `StringValue` and friends. Read the upgrade notes before you
roll it out.

### Upgrade notes

- **Some targeting rules match different users (qfg-goi1.5.2).** A context
  version of `"2"` or `"2.0"` (common for mobile app versions) now compares as
  `2.0.0` in semver rules instead of failing every one. A numeric context
  value of 1e6 or more (for example a JSON-decoded id in an
  `openfeature.EvaluationContext`) now matches an `is one of ["1234567"]`
  list, and a list holding an exponent string (`"1.234567e+06"`) stops
  matching. Pass dates to `PROP_BEFORE`/`PROP_AFTER` rules as epoch
  milliseconds or ISO 8601 / RFC 3339 with `Z` or a numeric offset;
  date-only and offset-less strings never match.
- **Some users move to a different weighted variant once (qfg-goi1.9).** When
  a rollout hashes on a numeric property whose value is a whole-number float
  of 1e6 or more, the hash input is now plain digits (`1234567`), matching
  sdk-node and sdk-ruby. Those users are re-bucketed once on upgrade (the
  OpenFeature reason stays `SPLIT`). Integer-typed values and values below
  1e6 are unaffected.
- **A rejected SDK key backs off (qfg-goi1.10.23).** When fetches get 401,
  403, 404 or 410, the underlying client stops SSE and polling and probes
  once every 150-300s instead, logging one ERROR (status and host, never the
  key). Evaluations keep serving the last good config; the first valid
  response resumes normal streaming. The provider emits no extra OpenFeature
  event for this.

### Also inherited

- Non-confidential ENV_VAR-provided values now report their resolved value in
  evaluation-summary telemetry; `confidential: true` values stay redacted
  (qfg-goi1.10.16).
- The fallback poller re-engages after a recovered panic instead of staying
  silent for the rest of an SSE outage (qfg-goi1.10.14).

## 1.3.0 - 2026-10-09

- Bump the `github.com/quonfig/sdk-go` pin from v1.4.0 to v1.6.0 to inherit the
  SDK quality program's Wave 0/1 fixes: telemetry context deep copy (no
  concurrent-map crash, cycle-safe, UUID and scalar arrays kept), ENV_VAR
  errors no longer leak values, SSE header timeout plus 401/403 WARN,
  state-callback panic recovery (qfg-goi1.2.4, qfg-goi1.2.44, qfg-goi1.2.46,
  qfg-goi1.1.1). `WithConfigFetchTimeout` is deprecated upstream; this
  provider does not call it. No change to this provider's behavior.

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
