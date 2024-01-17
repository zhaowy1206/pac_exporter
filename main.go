package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("PAC Meter")
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <command> [port]")
		return
	}

	command := os.Args[1]

	switch command {
	case "add":
		if err := AddMetricConfigInteractive("pac_exporter.json"); err != nil {
			log.Fatalf("Failed to add metric: %s", err)
		}
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: program run <port>")
			return
		}
		port := os.Args[2]
		run(port)
	default:
		fmt.Println("Unknown command. Use 'add' to add a new metric or 'run' to start hosting metrics.")
	}
}

func AddMetricConfigInteractive(file string) error {
	reader := bufio.NewReader(os.Stdin)

	// Prompt the user to input the fields of the new metric
	fmt.Print("Enter metric name: ")
	name, _ := reader.ReadString('\n')

	fmt.Print("Enter metric description: ")
	description, _ := reader.ReadString('\n')

	fmt.Print("Enter metric script name: ")
	scriptName, _ := reader.ReadString('\n')

	fmt.Print("Enter metric unit: ")
	unit, _ := reader.ReadString('\n')

	// Create a new MetricConfig with the user's inputs
	newMetric := MetricConfig{
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Type:        "gauge",
		ScriptName:  strings.TrimSpace(scriptName),
		Unit:        strings.TrimSpace(unit),
	}

	// Validate the new metric and add it to the configuration
	return AddMetricConfig(file, newMetric)
}

func run(port string) (err error) {
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

	// Initialize the metrics
	err = InitMetrics(meter, "pac_exporter.json")
	if err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	// Run serveMetrics in a goroutine
	go serveMetrics(port)
	// Wait for interruption.
	<-ctx.Done()
	// Stop receiving signal notifications as soon as possible.
	stop()

	return
}

func InitMetrics(meter metric.Meter, file string) error {
	// Load the configuration
	config, err := LoadConfig(file)
	if err != nil {
		return err
	}

	// Iterate over the metrics in the configuration
	for _, metricConfig := range config.Metrics {
		// Initialize the metric
		if _, err := meter.Int64ObservableGauge(metricConfig.Name,
			metric.WithDescription(metricConfig.Description),
			metric.WithUnit(metricConfig.Unit),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				// Execute the script
				out, err := exec.Command(metricConfig.ScriptName).Output()
				if err != nil {
					log.Printf("Failed to execute script %s: %v", metricConfig.ScriptName, err)
					return nil
				}

				// Parse the output to a float64
				percent, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
				if err != nil {
					log.Printf("Failed to parse output of script %s: %v", metricConfig.ScriptName, err)
					return nil
				}

				// Observe the result
				o.Observe(int64(percent))
				return nil
			}),
		); err != nil {
			return fmt.Errorf("failed to initialize metric %s: %w", metricConfig.Name, err)
		}
	}

	return nil
}
