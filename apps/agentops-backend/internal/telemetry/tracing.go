// Package telemetry provides Prometheus metrics and OpenTelemetry tracing.
package telemetry

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer configures the global OpenTelemetry tracer provider to export
// spans over OTLP/HTTP. It fails open: if the exporter cannot be created,
// it logs the error and returns a no-op shutdown function, so the API
// continues to serve traffic without tracing.
//
// The endpoint is read from OTEL_EXPORTER_OTLP_ENDPOINT. If unset, it
// defaults to http://localhost:4318 (Jaeger's OTLP/HTTP port).
func InitTracer() (shutdown func(context.Context) error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:4318"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint+"/v1/traces"),
		otlptracehttp.WithInsecure(), // local Jaeger has no TLS
	)
	if err != nil {
		log.Printf("telemetry: OTLP exporter disabled: %v", err)
		return func(context.Context) error { return nil }
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("chronobunker-api"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		log.Printf("telemetry: resource detection failed: %v", err)
		res = resource.Default()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Propagate W3C TraceContext and Baggage headers. Without this, the
	// otelhttp middleware cannot extract the traceparent header sent by the
	// Enterprise Gateway, and the Core's spans are orphaned from the Gateway's
	// trace. With it, the Jaeger UI shows a single trace spanning both services.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("telemetry: OTLP tracing enabled, exporting to %s", endpoint)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}
