package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	stepIO "github.com/machship/step-essentials/io"
)

// HTTPClient interface for mocking HTTP requests
type HTTPClient interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}

// Config holds the configuration for the request
type Config struct {
	ConnectionsURL string
	TargetURL      string
	Headers        map[string]string
}

// App holds the dependencies
type App struct {
	client HTTPClient
	config Config
}

// NewApp creates a new App instance
func NewApp(client HTTPClient, config Config) *App {
	return &App{
		client: client,
		config: config,
	}
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		ConnectionsURL: "http://localhost:8081/api/v1/connections/send",
		TargetURL:      "https://httpbin.org/get",
		Headers: map[string]string{
			"User-Agent": "Visual-Go-Test/1.0",
			"Accept":     "application/json",
		},
	}
}

// preparePayload creates the request payload
func (a *App) preparePayload() ([]byte, error) {
	payload := map[string]interface{}{
		"method":  "GET",
		"url":     a.config.TargetURL,
		"headers": a.config.Headers,
	}

	return json.Marshal(payload)
}

// sendRequest sends the HTTP request to the connections service
func (a *App) sendRequest(payload []byte) (*http.Response, error) {
	return a.client.Post(a.config.ConnectionsURL, "application/json", bytes.NewBuffer(payload))
}

// parseResponse parses the response from the connections service
func (a *App) parseResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()

	var result map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

// setOutputs sets the step outputs
func (a *App) setOutputs(result map[string]interface{}) {
	stepIO.SetOutputs(map[string]any{
		"status_code":   result["status_code"],
		"response_body": result["body"],
		"duration":      result["duration"],
	})
}

// Run executes the main logic
func (a *App) Run() error {
	// Prepare request payload
	payload, err := a.preparePayload()
	if err != nil {
		return fmt.Errorf("failed to prepare payload: %w", err)
	}

	// Send request to connections service
	resp, err := a.sendRequest(payload)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Parse response
	result, err := a.parseResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Set outputs
	a.setOutputs(result)
	return nil
}

func main() {
	app := NewApp(&http.Client{}, DefaultConfig())

	if err := app.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
