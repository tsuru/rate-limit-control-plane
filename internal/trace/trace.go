package trace

import (
	"log"

	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	TraceProvider *sdktrace.TracerProvider
	Trace         trace.Tracer
)

func InitTrace(url string) func() {
	exporter, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(url),
		),
	)
	if err != nil {
		log.Fatalf("failed to create jaeger exporter: %v", err)
	}
	TraceProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("data_test"),
		)),
	)
	Trace = TraceProvider.Tracer("data_test")
	return func() {

	}
}
