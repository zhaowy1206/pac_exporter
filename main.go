package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter       = otel.Meter("systemUsage")
	cpuUsage    metric.Int64ObservableGauge
	memoryUsage metric.Int64ObservableGauge
)

func init() {

}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up OpenTelemetry.
	serviceName := "PAC Metrics"
	serviceVersion := "1.0"
	otelShutdown, err := setupOTelSDK(ctx, serviceName, serviceVersion)
	if err != nil {
		return
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Run recordCPUUsage in a goroutine.
	go serveMetrics()

	// Wait for interruption.
	<-ctx.Done()
	// Stop receiving signal notifications as soon as possible.
	stop()

	return
}
