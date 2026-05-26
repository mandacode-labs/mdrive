package telemetry

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"github.com/mandacode-labs/retrowin-go/internal/config"
)

// mockTraceCollector implements TraceServiceServer.
type mockTraceCollector struct {
	collectortrace.UnimplementedTraceServiceServer
	spanCount int
}

func (m *mockTraceCollector) Export(_ context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			m.spanCount += len(ss.GetSpans())
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

// mockMetricsCollector implements MetricsServiceServer.
type mockMetricsCollector struct {
	collectormetrics.UnimplementedMetricsServiceServer
	metricCount int
}

func (m *mockMetricsCollector) Export(_ context.Context, req *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	for _, rm := range req.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			m.metricCount += len(sm.GetMetrics())
		}
	}
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

// startMockCollector starts a mock OTLP gRPC collector on a random port.
func startMockCollector(t *testing.T) (*mockTraceCollector, *mockMetricsCollector, string) {
	t.Helper()

	traceCollector := &mockTraceCollector{}
	metricsCollector := &mockMetricsCollector{}

	srv := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(srv, traceCollector)
	collectormetrics.RegisterMetricsServiceServer(srv, metricsCollector)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.GracefulStop)

	return traceCollector, metricsCollector, ln.Addr().String()
}

func TestNewProviders_Disabled(t *testing.T) {
	cfg := &config.Config{
		Telemetry: config.TelemetryConfig{
			Enabled: false,
		},
	}

	providers, err := NewProviders(cfg)
	require.NoError(t, err)
	require.NotNil(t, providers)

	assert.NoError(t, providers.Shutdown(context.Background()))
}

func TestNewProviders_EnabledWithMockCollector(t *testing.T) {
	traceCol, metricsCol, addr := startMockCollector(t)

	cfg := &config.Config{
		App: config.AppConfig{
			Version: "test",
			Env:     "test",
		},
		Telemetry: config.TelemetryConfig{
			Enabled:     true,
			Endpoint:    addr,
			Insecure:    true,
			ServiceName: "retrowin-test",
		},
	}

	providers, err := NewProviders(cfg)
	require.NoError(t, err)
	require.NotNil(t, providers)

	assert.NotNil(t, providers.TracerProvider)
	assert.NotNil(t, providers.MeterProvider)

	// Create a span and record a metric
	tracer := providers.TracerProvider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	span.End()

	meter := providers.MeterProvider.Meter("test")
	counter, _ := meter.Int64Counter("test.counter")
	counter.Add(ctx, 1)

	// Shutdown flushes to the mock collector
	assert.NoError(t, providers.Shutdown(context.Background()))

	// Verify the mock collector received data
	assert.Equal(t, 1, traceCol.spanCount, "expected 1 trace span")
	assert.Equal(t, 1, metricsCol.metricCount, "expected 1 metric")
}

func TestNewProviders_ResourceAttributes(t *testing.T) {
	traceCol, _, addr := startMockCollector(t)

	cfg := &config.Config{
		App: config.AppConfig{
			Version: "1.2.3",
			Env:     "staging",
		},
		Telemetry: config.TelemetryConfig{
			Enabled:     true,
			Endpoint:    addr,
			Insecure:    true,
			ServiceName: "my-service",
		},
	}

	providers, err := NewProviders(cfg)
	require.NoError(t, err)
	require.NotNil(t, providers)

	tracer := providers.TracerProvider.Tracer("test")
	_, span := tracer.Start(context.Background(), "attr-test")
	span.End()

	assert.NoError(t, providers.Shutdown(context.Background()))
	assert.True(t, traceCol.spanCount > 0, "expected at least 1 span")
}

func TestProviders_Shutdown_Nil(t *testing.T) {
	p := &Providers{}
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestProviders_Shutdown_Idempotent(t *testing.T) {
	_, _, addr := startMockCollector(t)

	cfg := &config.Config{
		App: config.AppConfig{Version: "test", Env: "test"},
		Telemetry: config.TelemetryConfig{
			Enabled:     true,
			Endpoint:    addr,
			Insecure:    true,
			ServiceName: "retrowin-test",
		},
	}

	providers, err := NewProviders(cfg)
	require.NoError(t, err)

	assert.NoError(t, providers.Shutdown(context.Background()))
	// Second shutdown may return an error from already-shutdown providers, but must not panic
	_ = providers.Shutdown(context.Background())
}

func TestBuildTLSConfig_NoCA(t *testing.T) {
	tlsCfg, err := buildTLSConfig("")
	assert.NoError(t, err)
	assert.Nil(t, tlsCfg)
}

func TestBuildTLSConfig_InvalidPath(t *testing.T) {
	_, err := buildTLSConfig("/nonexistent/ca.crt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read CA cert")
}

func TestBuildTLSConfig_InvalidPEM(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "ca-*.crt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, _ = tmpFile.WriteString("not a valid PEM")
	_ = tmpFile.Close()

	_, err = buildTLSConfig(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse CA cert")
}
