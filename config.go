package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type MetricConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	ScriptName  string `json:"script_name"`
	Unit        string `json:"unit"`
}

type Config struct {
	Metrics []MetricConfig `json:"metrics"`
}

func LoadConfig(file string) (*Config, error) {
	configFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	var config Config
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (m *MetricConfig) ExecuteScript() (int, error) {
	out, err := exec.Command(m.ScriptName).Output()
	if err != nil {
		return 0, err
	}

	result, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}

	return result, nil
}

func AddMetricConfig(file string, newMetric MetricConfig) error {
	var config Config

	// Try to load the existing configuration
	_, err := os.Stat(file)
	if err == nil {
		configPtr, err := LoadConfig(file)
		if err != nil {
			return err
		}
		config = *configPtr
	}

	// Validate the type in the new metric
	err = newMetric.ValidateType()
	if err != nil {
		return fmt.Errorf("invalid type in new metric: %w", err)
	}

	// Validate the script in the new metric
	err = newMetric.ValidateScript()
	if err != nil {
		return fmt.Errorf("invalid script in new metric: %w", err)
	}

	// Add the new metric to the configuration
	config.Metrics = append(config.Metrics, newMetric)

	// Open the configuration file for writing
	configFile, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer configFile.Close()

	// Write the updated configuration back to the file
	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

func (m *MetricConfig) ValidateScript() error {
	// Check if the script file exists
	_, err := os.Stat(m.ScriptName)
	if err != nil {
		return fmt.Errorf("script file does not exist: %w", err)
	}

	// Execute the script and check that it returns a number
	_, err = m.ExecuteScript()
	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

func (m *MetricConfig) ValidateType() error {
	if m.Type != "gauge" {
		return fmt.Errorf("invalid type: %s, only 'gauge' is allowed", m.Type)
	}
	return nil
}
