# github.com/quonfig/openfeature-go

OpenFeature provider for [Quonfig](https://quonfig.com) — wraps the
[github.com/quonfig/sdk-go](https://github.com/quonfig/sdk-go) native SDK.

## Installation

```bash
go get github.com/quonfig/openfeature-go
go get github.com/open-feature/go-sdk
```

## Usage

```go
import (
    openfeaturego "github.com/quonfig/openfeature-go"
    "github.com/open-feature/go-sdk/openfeature"
)

provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
    SDKKey:              "qf_sk_production_...",
    TargetingKeyMapping: "user.id",  // default
})

// Register and wait for initialization
if err := openfeature.SetProviderAndWait(provider); err != nil {
    log.Fatal(err)
}

client := openfeature.NewDefaultClient()
ctx := context.Background()

// Boolean flag
enabled, err := client.BooleanValue(ctx, "my-flag", false, openfeature.EvaluationContext{})

// String flag with context
evalCtx := openfeature.NewEvaluationContext("user-123", map[string]any{
    "user.email": "alice@co.com",
    "org.tier":   "enterprise",
})
plan, err := client.StringValue(ctx, "billing.plan", "free", evalCtx)
```

## Context mapping

OpenFeature uses a flat key-value evaluation context. Quonfig uses a nested
namespace model. The provider maps between them using dot-notation:

| OpenFeature key | Quonfig namespace | Quonfig property |
|----------------|-------------------|-----------------|
| `"user.email"` | `"user"` | `"email"` |
| `"org.tier"` | `"org"` | `"tier"` |
| `"country"` (no dot) | `""` (default) | `"country"` |
| `"user.ip.address"` | `"user"` | `"ip.address"` (split on first dot only) |
| `targetingKey` | `"user"` | `"id"` (via `TargetingKeyMapping`) |

To use a different targeting key property:

```go
provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
    SDKKey:              "qf_sk_...",
    TargetingKeyMapping: "org.id",  // targetingKey -> org namespace, id property
})
```

## Local / offline mode

Use `DataDir` instead of `SDKKey` to load config from a local directory:

```go
provider := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
    DataDir:     "/path/to/workspace",
    Environment: "Production",
})
```

## Native SDK escape hatch

For features not available in OpenFeature (duration values, log levels, etc.):

```go
nativeClient := provider.GetClient()
dur, found, err := nativeClient.GetDurationValue("cache.ttl", nil)
```

## What you lose vs. the native SDK

The OpenFeature spec covers common flag types. Some Quonfig-native features
require the native SDK directly:

1. **Log levels** (`shouldLog`, `logger`) -- native SDK only
2. **`string_list` configs** must be accessed via `ObjectValue` and cast to `[]any`
3. **`duration` configs** are not accessible via OpenFeature (use `GetClient()`)
4. **`bytes` configs** are not accessible via OpenFeature
5. **`keys()`** and raw config access -- native SDK only
6. Context keys must use dot-notation (`"user.email"`, not nested objects)
7. `targetingKey` maps to `user.id` by default

## Type mapping

| Quonfig type | OpenFeature method | Notes |
|-------------|--------------------|-------|
| `bool` | `BooleanValue` | Direct |
| `string` | `StringValue` | Direct |
| `int` | `IntValue` | Returns `int64` |
| `double` | `FloatValue` | Returns `float64` |
| `string_list` | `ObjectValue` | Returns `[]any` |
| `json` | `ObjectValue` | Returns parsed JSON |
| `duration` | N/A | Use native client |
| `log_level` | N/A | Native SDK only |
