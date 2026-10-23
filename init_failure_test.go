package openfeaturego_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	openfeaturego "github.com/quonfig/openfeature-go/v2"
	quonfig "github.com/quonfig/sdk-go/v2"
)

// Startup-failure mapping (qfg-9p4i.12, design doc B.5):
//
//	fail     -> Init returns *quonfig.InitError; status ERROR. sdk-go returns no
//	            client under fail (Decision 4), so the restart is the retry.
//	wait     -> Init blocks; status NOT_READY until the first config.
//	continue -> Init returns nil, then PROVIDER_ERROR (status ERROR) while the
//	            client has no config; PROVIDER_READY when it heals.

// switchableDelivery answers 503 until up is set, then serves one config.
type switchableDelivery struct {
	up  atomic.Bool
	srv *httptest.Server
}

func newSwitchableDelivery(t *testing.T) *switchableDelivery {
	t.Helper()
	d := &switchableDelivery{}
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !d.up.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(quonfig.ConfigEnvelope{
			Configs: []quonfig.ConfigResponse{{
				Key:       "live.flag",
				Type:      quonfig.ConfigTypeFeatureFlag,
				ValueType: quonfig.ValueTypeBool,
				Default: quonfig.RuleSet{Rules: []quonfig.Rule{{
					Value: quonfig.Value{Type: quonfig.ValueTypeBool, Value: true},
				}}},
			}},
			Meta: quonfig.Meta{Version: "v-live", Environment: "Production", Generation: 3},
		})
	}))
	t.Cleanup(d.srv.Close)
	return d
}

var domainSeq atomic.Int64

// uniqueDomain names a fresh OpenFeature domain per run, so -count=N does not
// inherit the previous run's provider status.
func uniqueDomain(t *testing.T) string {
	return fmt.Sprintf("%s-%d", t.Name(), domainSeq.Add(1))
}

func deliveryProvider(t *testing.T, d *switchableDelivery, extra ...quonfig.Option) *openfeaturego.QuonfigProvider {
	t.Helper()
	opts := []quonfig.Option{
		quonfig.WithAPIURLs([]string{d.srv.URL}),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithQuonfigUserContext(false),
	}
	p := openfeaturego.NewQuonfigProvider(openfeaturego.Options{
		SDKKey:            "test-sdk-key",
		AdditionalOptions: append(opts, extra...),
	})
	t.Cleanup(p.Shutdown)
	return p
}

func TestInitFailure_Fail_InitErrorAndStatusError(t *testing.T) {
	d := newSwitchableDelivery(t)
	p := deliveryProvider(t, d, quonfig.WithInitTimeout(300*time.Millisecond))
	domain := uniqueDomain(t)

	err := openfeature.SetNamedProviderAndWait(domain, p)
	require.Error(t, err)
	var ie *quonfig.InitError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, quonfig.InitErrorUnavailable, ie.Reason)

	client := openfeature.NewClient(domain)
	assert.Equal(t, openfeature.ErrorState, client.State())
	assert.Nil(t, p.GetClient(), "sdk-go returns no client under fail")

	details, _ := client.BooleanValueDetails(context.Background(), "live.flag", false, openfeature.EvaluationContext{})
	assert.False(t, details.Value)
	assert.Equal(t, openfeature.ProviderNotReadyCode, details.ErrorCode)
}

func TestInitFailure_Continue_ErrorUntilHealThenReady(t *testing.T) {
	d := newSwitchableDelivery(t)
	p := deliveryProvider(t, d,
		quonfig.WithOnInitFailure(quonfig.InitFailureContinue),
		quonfig.WithInitTimeout(200*time.Millisecond))
	domain := uniqueDomain(t)

	require.NoError(t, openfeature.SetNamedProviderAndWait(domain, p), "continue resolves")
	client := openfeature.NewClient(domain)

	require.Eventually(t, func() bool { return client.State() == openfeature.ErrorState },
		2*time.Second, 10*time.Millisecond, "continue without config reports ERROR")

	details, _ := client.BooleanValueDetails(context.Background(), "live.flag", false, openfeature.EvaluationContext{})
	assert.False(t, details.Value, "code default while not ready")
	assert.Equal(t, openfeature.ProviderNotReadyCode, details.ErrorCode)

	var readyEvents atomic.Int32
	cb := func(openfeature.EventDetails) { readyEvents.Add(1) }
	client.AddHandler(openfeature.ProviderReady, &cb)
	t.Cleanup(func() { client.RemoveHandler(openfeature.ProviderReady, &cb) })

	d.up.Store(true)
	require.Eventually(t, func() bool { return client.State() == openfeature.ReadyState },
		5*time.Second, 10*time.Millisecond, "heal flips the status to READY")
	require.Eventually(t, func() bool { return readyEvents.Load() >= 1 },
		2*time.Second, 10*time.Millisecond, "heal emits PROVIDER_READY")

	v, err := client.BooleanValue(context.Background(), "live.flag", false, openfeature.EvaluationContext{})
	require.NoError(t, err)
	assert.True(t, v, "live config after heal")
}

func TestInitFailure_Wait_NotReadyUntilFirstConfig(t *testing.T) {
	d := newSwitchableDelivery(t)
	p := deliveryProvider(t, d,
		quonfig.WithOnInitFailure(quonfig.InitFailureWait),
		quonfig.WithInitTimeout(100*time.Millisecond))
	domain := uniqueDomain(t)

	require.NoError(t, openfeature.SetNamedProvider(domain, p))
	client := openfeature.NewClient(domain)

	time.Sleep(400 * time.Millisecond) // well past initTimeout: wait keeps blocking
	assert.Equal(t, openfeature.NotReadyState, client.State())
	_, err := client.BooleanValue(context.Background(), "live.flag", false, openfeature.EvaluationContext{})
	assert.ErrorIs(t, err, openfeature.ProviderNotReadyError)

	d.up.Store(true)
	require.Eventually(t, func() bool { return client.State() == openfeature.ReadyState },
		5*time.Second, 10*time.Millisecond)
	v, err := client.BooleanValue(context.Background(), "live.flag", false, openfeature.EvaluationContext{})
	require.NoError(t, err)
	assert.True(t, v)
}
