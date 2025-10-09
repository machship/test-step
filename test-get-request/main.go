package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/machship/step-essentials/io"
)

func main() {
	// Prepare request to connections service
	payload := map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/get",
		"headers": map[string]string{
			"User-Agent": "Visual-Go-Test/1.0",
			"Accept":     "application/json",
		},
	}

	jsonData, _ := json.Marshal(payload)

	// Send request to connections service
	resp, err := http.Post("http://localhost:8081/api/v1/connections/send",
		"application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	io.SetOutputs(map[string]any{
		"status_code":   result["status_code"],
		"response_body": result["body"],
		"duration":      result["duration"],
	})
}
