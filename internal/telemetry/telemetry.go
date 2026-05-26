package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"

	"github.com/mandacode-labs/retrowin-go/internal/config"
)

// Providers holds the initialized OTel providers and a shutdown function.
type Providers struct {
	TracerProvider trace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	shutdown       func(ctx context.Context) error
}

// Shutdown flushes and shuts down all OTel providers.
func (p *Providers) Shutdown(ctx context.Context) error {
	if p.shutdown != nil {
		return p.shutdown(ctx)
	}
	return nil
}

var once sync.Once

// NewProviders creates and registers OTel providers based on config.
// Returns a no-op Providers when telemetry is disabled.
func NewProviders(cfg *config.Config) (*Providers, error) {
	if !cfg.Telemetry.Enabled {
		return &Providers{}, nil
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.Telemetry.ServiceName),
			semconv.ServiceVersionKey.String(cfg.App.Version),
			semconv.DeploymentEnvironmentKey.String(cfg.App.Env),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	tlsCfg, err := buildTLSConfig(cfg.Telemetry.CACert)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	var traceOpts []otlptracegrpc.Option
	if cfg.Telemetry.Insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	} else if tlsCfg != nil {
		traceOpts = append(traceOpts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg)))
	}

	traceExporter, err := otlptracegrpc.New(context.Background(),
		append(traceOpts, otlptracegrpc.WithEndpoint(cfg.Telemetry.Endpoint))...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	var metricOpts []otlpmetricgrpc.Option
	if cfg.Telemetry.Insecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	} else if tlsCfg != nil {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg)))
	}

	metricExporter, err := otlpmetricgrpc.New(context.Background(),
		append(metricOpts, otlpmetricgrpc.WithEndpoint(cfg.Telemetry.Endpoint))...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	// Register as global providers so ogen and other libraries pick them up.
	once.Do(func() {
		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
	})

	return &Providers{
		TracerProvider: tp,
		MeterProvider:  mp,
		shutdown: func(ctx context.Context) error {
			var errs []error
			if err := tp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
			}
			if err := mp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
			}
			if len(errs) > 0 {
				return fmt.Errorf("telemetry shutdown errors: %v", errs)
			}
			return nil
		},
	}, nil
}

// buildTLSConfig creates a *tls.Config for the OTLP gRPC connection.
// Returns nil if no custom CA is configured (uses system CA pool).
func buildTLSConfig(caCertPath string) (*tls.Config, error) {
	if caCertPath == "" {
		return nil, nil
	}

	caPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert %s: %w", caCertPath, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA cert from %s", caCertPath)
	}

	return &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}, nil
}
