package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// MockHTTPClient implements HTTPClient for testing
type MockHTTPClient struct {
	PostFunc func(url, contentType string, body io.Reader) (*http.Response, error)
}

func (m *MockHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	if m.PostFunc != nil {
		return m.PostFunc(url, contentType, body)
	}
	return nil, errors.New("mock not implemented")
}

// MockResponse creates a mock HTTP response
func MockResponse(statusCode int, body map[string]interface{}) *http.Response {
	jsonBody, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(string(jsonBody))),
		Header:     make(http.Header),
	}
}

func TestNewApp(t *testing.T) {
	client := &MockHTTPClient{}
	config := DefaultConfig()

	app := NewApp(client, config)

	if app.client != client {
		t.Error("Expected client to be set correctly")
	}

	if !reflect.DeepEqual(app.config, config) {
		t.Error("Expected config to be set correctly")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	expectedURL := "http://localhost:8081/api/v1/connections/send"
	if config.ConnectionsURL != expectedURL {
		t.Errorf("Expected ConnectionsURL to be %s, got %s", expectedURL, config.ConnectionsURL)
	}

	expectedTargetURL := "https://httpbin.org/get"
	if config.TargetURL != expectedTargetURL {
		t.Errorf("Expected TargetURL to be %s, got %s", expectedTargetURL, config.TargetURL)
	}

	expectedHeaders := map[string]string{
		"User-Agent": "Visual-Go-Test/1.0",
		"Accept":     "application/json",
	}
	if !reflect.DeepEqual(config.Headers, expectedHeaders) {
		t.Errorf("Expected headers %v, got %v", expectedHeaders, config.Headers)
	}
}

func TestApp_preparePayload(t *testing.T) {
	config := Config{
		TargetURL: "https://example.com/test",
		Headers: map[string]string{
			"Authorization": "Bearer token",
			"Content-Type":  "application/json",
		},
	}

	app := NewApp(&MockHTTPClient{}, config)

	payload, err := app.preparePayload()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	expectedMethod := "GET"
	if result["method"] != expectedMethod {
		t.Errorf("Expected method %s, got %v", expectedMethod, result["method"])
	}

	if result["url"] != config.TargetURL {
		t.Errorf("Expected url %s, got %v", config.TargetURL, result["url"])
	}

	headers, ok := result["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected headers to be a map")
	}

	for key, expectedValue := range config.Headers {
		if headers[key] != expectedValue {
			t.Errorf("Expected header %s to be %s, got %v", key, expectedValue, headers[key])
		}
	}
}

func TestApp_sendRequest(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *http.Response
		mockError     error
		expectError   bool
		expectedURL   string
		expectedCType string
	}{
		{
			name:          "successful request",
			mockResponse:  MockResponse(200, map[string]interface{}{"success": true}),
			mockError:     nil,
			expectError:   false,
			expectedURL:   "http://localhost:8081/api/v1/connections/send",
			expectedCType: "application/json",
		},
		{
			name:         "request error",
			mockResponse: nil,
			mockError:    errors.New("network error"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedURL, capturedContentType string
			var capturedBody []byte

			mockClient := &MockHTTPClient{
				PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
					capturedURL = url
					capturedContentType = contentType
					if body != nil {
						capturedBody, _ = io.ReadAll(body)
					}
					return tt.mockResponse, tt.mockError
				},
			}

			app := NewApp(mockClient, DefaultConfig())
			payload := []byte(`{"test": "data"}`)

			resp, err := app.sendRequest(payload)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp != tt.mockResponse {
				t.Error("Expected mock response to be returned")
			}

			if capturedURL != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, capturedURL)
			}

			if capturedContentType != tt.expectedCType {
				t.Errorf("Expected content type %s, got %s", tt.expectedCType, capturedContentType)
			}

			if !bytes.Equal(capturedBody, payload) {
				t.Errorf("Expected payload %s, got %s", string(payload), string(capturedBody))
			}
		})
	}
}

func TestApp_parseResponse(t *testing.T) {
	tests := []struct {
		name         string
		responseBody map[string]interface{}
		expectError  bool
	}{
		{
			name: "valid response",
			responseBody: map[string]interface{}{
				"status_code": 200,
				"body":        "success",
				"duration":    "100ms",
			},
			expectError: false,
		},
		{
			name:         "empty response",
			responseBody: map[string]interface{}{},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&MockHTTPClient{}, DefaultConfig())
			resp := MockResponse(200, tt.responseBody)

			result, err := app.parseResponse(resp)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			// Compare individual fields instead of using reflect.DeepEqual due to type conversion issues
			if len(result) != len(tt.responseBody) {
				t.Errorf("Expected result length %d, got %d", len(tt.responseBody), len(result))
				return
			}

			for key, expectedValue := range tt.responseBody {
				actualValue := result[key]

				// Handle type conversion for numeric values
				switch exp := expectedValue.(type) {
				case int:
					if act, ok := actualValue.(float64); ok {
						if float64(exp) != act {
							t.Errorf("Expected result[%s] = %v, got %v", key, expectedValue, actualValue)
						}
					} else if actualValue != expectedValue {
						t.Errorf("Expected result[%s] = %v, got %v", key, expectedValue, actualValue)
					}
				default:
					if actualValue != expectedValue {
						t.Errorf("Expected result[%s] = %v, got %v", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestApp_parseResponse_InvalidJSON(t *testing.T) {
	app := NewApp(&MockHTTPClient{}, DefaultConfig())

	// Create response with invalid JSON
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("invalid json")),
		Header:     make(http.Header),
	}

	_, err := app.parseResponse(resp)
	if err == nil {
		t.Error("Expected error for invalid JSON, got none")
	}
}

func TestApp_Run_Success(t *testing.T) {
	responseBody := map[string]interface{}{
		"status_code": 200,
		"body":        `{"message": "success"}`,
		"duration":    "150ms",
	}

	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			return MockResponse(200, responseBody), nil
		},
	}

	app := NewApp(mockClient, DefaultConfig())

	err := app.Run()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestApp_Run_SendRequestError(t *testing.T) {
	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}

	app := NewApp(mockClient, DefaultConfig())

	err := app.Run()
	if err == nil {
		t.Error("Expected error, got none")
	}

	if !strings.Contains(err.Error(), "failed to send request") {
		t.Errorf("Expected error message to contain 'failed to send request', got %v", err)
	}
}

func TestApp_Run_ParseResponseError(t *testing.T) {
	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			// Return response with invalid JSON
			resp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("invalid json")),
				Header:     make(http.Header),
			}
			return resp, nil
		},
	}

	app := NewApp(mockClient, DefaultConfig())

	err := app.Run()
	if err == nil {
		t.Error("Expected error, got none")
	}

	if !strings.Contains(err.Error(), "failed to parse response") {
		t.Errorf("Expected error message to contain 'failed to parse response', got %v", err)
	}
}

func TestApp_Run_Integration(t *testing.T) {
	// Integration test that verifies the entire flow
	expectedPayload := map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/get",
		"headers": map[string]interface{}{
			"User-Agent": "Visual-Go-Test/1.0",
			"Accept":     "application/json",
		},
	}

	responseBody := map[string]interface{}{
		"status_code": 200,
		"body":        `{"message": "Hello, World!"}`,
		"duration":    "95ms",
	}

	var receivedPayload map[string]interface{}

	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			// Capture and verify the payload
			bodyBytes, _ := io.ReadAll(body)
			json.Unmarshal(bodyBytes, &receivedPayload)

			// Verify the request parameters
			expectedURL := "http://localhost:8081/api/v1/connections/send"
			if url != expectedURL {
				t.Errorf("Expected URL %s, got %s", expectedURL, url)
			}

			if contentType != "application/json" {
				t.Errorf("Expected content type application/json, got %s", contentType)
			}

			return MockResponse(200, responseBody), nil
		},
	}

	app := NewApp(mockClient, DefaultConfig())

	err := app.Run()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the payload was constructed correctly
	if receivedPayload["method"] != expectedPayload["method"] {
		t.Errorf("Expected method %v, got %v", expectedPayload["method"], receivedPayload["method"])
	}

	if receivedPayload["url"] != expectedPayload["url"] {
		t.Errorf("Expected url %v, got %v", expectedPayload["url"], receivedPayload["url"])
	}

	// Check headers separately due to type conversion
	receivedHeaders, ok := receivedPayload["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected headers to be a map[string]interface{}")
	}

	expectedHeaders := expectedPayload["headers"].(map[string]interface{})
	for key, expectedValue := range expectedHeaders {
		if receivedHeaders[key] != expectedValue {
			t.Errorf("Expected header %s = %v, got %v", key, expectedValue, receivedHeaders[key])
		}
	}
}

// Benchmark tests
func BenchmarkApp_preparePayload(b *testing.B) {
	app := NewApp(&MockHTTPClient{}, DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := app.preparePayload()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApp_Run(b *testing.B) {
	responseBody := map[string]interface{}{
		"status_code": 200,
		"body":        `{"message": "success"}`,
		"duration":    "100ms",
	}

	mockClient := &MockHTTPClient{
		PostFunc: func(url, contentType string, body io.Reader) (*http.Response, error) {
			return MockResponse(200, responseBody), nil
		},
	}

	app := NewApp(mockClient, DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := app.Run()
		if err != nil {
			b.Fatal(err)
		}
	}
}
