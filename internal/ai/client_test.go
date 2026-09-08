package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paperMC_backend/internal/database"
)

func TestSanitizeLog(t *testing.T) {
	raw := `
[20:10:00 INFO]: Loading server from /home/marcin/Development/paperMC_backend/world
[20:10:01 INFO]: Windows path C:\Users\Administrator\AppData\Local\Temp\dump.txt
[20:10:02 INFO]: Config: rcon.password=SuperSecretPassword123! token: eyJhbGciOiJIUzI1NiJ9
[20:10:03 INFO]: Player UUID 550e8400-e29b-41d4-a716-446655440000 logged in from 192.168.1.100
[20:10:04 INFO]: Localhost connection from 127.0.0.1:25565 and bind to 0.0.0.0:25565
`
	workDir := "/home/marcin/Development/paperMC_backend"
	sanitized := SanitizeLog(raw, workDir)

	if strings.Contains(sanitized, "SuperSecretPassword123!") {
		t.Errorf("Password was not redacted: %s", sanitized)
	}
	if strings.Contains(sanitized, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("Token was not redacted: %s", sanitized)
	}
	if strings.Contains(sanitized, "550e8400-e29b-41d4-a716-446655440000") {
		t.Errorf("UUID was not redacted: %s", sanitized)
	}
	if strings.Contains(sanitized, "192.168.1.100") {
		t.Errorf("IP address was not redacted: %s", sanitized)
	}
	if strings.Contains(sanitized, "/home/marcin/Development/paperMC_backend") {
		t.Errorf("WorkDir was not redacted: %s", sanitized)
	}
	if strings.Contains(sanitized, "C:\\Users\\Administrator") {
		t.Errorf("Windows user path was not redacted: %s", sanitized)
	}

	// Preserved entities
	if !strings.Contains(sanitized, "127.0.0.1") {
		t.Errorf("Expected 127.0.0.1 to be preserved: %s", sanitized)
	}
	if !strings.Contains(sanitized, "0.0.0.0") {
		t.Errorf("Expected 0.0.0.0 to be preserved: %s", sanitized)
	}
	if !strings.Contains(sanitized, "[SERVER_DIR]") {
		t.Errorf("Expected [SERVER_DIR] placeholder: %s", sanitized)
	}

	// Empty string test
	if SanitizeLog("", "") != "" {
		t.Errorf("Expected empty string for empty input")
	}
}

func TestExplainCrashDisabledAndMissingKey(t *testing.T) {
	client := NewClient()
	report := &database.CrashReport{
		Category: "OutOfMemory",
		Title:    "OOM",
		RawLog:   "java.lang.OutOfMemoryError",
	}

	// Disabled
	_, err := client.ExplainCrash(context.Background(), &database.AISettings{IsEnabled: false}, report, "")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Expected disabled error, got: %v", err)
	}

	// Missing API key
	_, err = client.ExplainCrash(context.Background(), &database.AISettings{
		IsEnabled: true,
		Provider:  "openai",
		APIKey:    "",
		BaseURL:   "https://api.openai.com",
	}, report, "")
	if err == nil || !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("Expected missing API key error, got: %v", err)
	}

	// Unsupported provider
	_, err = client.ExplainCrash(context.Background(), &database.AISettings{
		IsEnabled: true,
		Provider:  "unknown_provider",
		APIKey:    "test-key",
	}, report, "")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("Expected unsupported provider error, got: %v", err)
	}
}

func TestExplainCrashOpenAISuccess(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Missing or invalid Authorization header: %s", r.Header.Get("Authorization"))
		}

		var req OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode OpenAI request: %v", err)
		}

		resp := OpenAIChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "### Diagnosis\nThe server ran out of memory. Increase RAM to 8GB.",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewClient()
	settings := &database.AISettings{
		IsEnabled: true,
		Provider:  "openai",
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "gpt-4o-mini",
	}

	report := &database.CrashReport{
		Category:       "OutOfMemory",
		Title:          "Server Out of Memory",
		Culprit:        "JVM Heap Exhaustion",
		Summary:        "OOM crash",
		Recommendation: "Increase RAM",
		RawLog:         strings.Repeat("java.lang.OutOfMemoryError: Java heap space\n", 200),
		CreatedAt:      time.Now(),
	}

	explanation, err := client.ExplainCrash(context.Background(), settings, report, "Why did this happen?")
	if err != nil {
		t.Fatalf("ExplainCrash failed: %v", err)
	}

	if !strings.Contains(explanation, "The server ran out of memory") {
		t.Errorf("Unexpected explanation response: %s", explanation)
	}
}

func TestExplainCrashOpenAIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := OpenAIChatResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{
				Message: "Incorrect API key provided",
				Type:    "invalid_request_error",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewClient()
	settings := &database.AISettings{
		IsEnabled: true,
		Provider:  "openai",
		APIKey:    "invalid-key",
		BaseURL:   mockServer.URL,
	}

	report := &database.CrashReport{
		Category: "OutOfMemory",
		RawLog:   "OOM",
	}

	_, err := client.ExplainCrash(context.Background(), settings, report, "")
	if err == nil || !strings.Contains(err.Error(), "Incorrect API key provided") {
		t.Errorf("Expected API error message, got: %v", err)
	}
}

func TestExplainCrashGeminiSuccess(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "key=gemini-secret-key") {
			t.Errorf("Expected key query param, got: %s", r.URL.RawQuery)
		}

		var req GeminiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode Gemini request: %v", err)
		}

		resp := GeminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{
				{
					Content: struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					}{
						Parts: []struct {
							Text string `json:"text"`
						}{
							{Text: "### Gemini Explanation\nPort 25565 is already occupied."},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewClient()
	settings := &database.AISettings{
		IsEnabled: true,
		Provider:  "gemini",
		APIKey:    "gemini-secret-key",
		BaseURL:   mockServer.URL,
		Model:     "gemini-1.5-flash",
	}

	report := &database.CrashReport{
		Category:       "PortConflict",
		Title:          "Port in use",
		Culprit:        "Port 25565",
		Summary:        "Address already in use",
		Recommendation: "Change port",
		RawLog:         "FAILED TO BIND TO PORT",
	}

	explanation, err := client.ExplainCrash(context.Background(), settings, report, "")
	if err != nil {
		t.Fatalf("ExplainCrash Gemini failed: %v", err)
	}

	if !strings.Contains(explanation, "Port 25565 is already occupied") {
		t.Errorf("Unexpected Gemini explanation: %s", explanation)
	}
}

func TestExplainCrashGeminiError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		resp := GeminiResponse{
			Error: &struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			}{
				Message: "API_KEY_INVALID",
				Code:    400,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewClient()
	settings := &database.AISettings{
		IsEnabled: true,
		Provider:  "gemini",
		APIKey:    "bad-gemini-key",
		BaseURL:   mockServer.URL,
	}

	report := &database.CrashReport{
		Category: "PortConflict",
		RawLog:   "bind failed",
	}

	_, err := client.ExplainCrash(context.Background(), settings, report, "")
	if err == nil || !strings.Contains(err.Error(), "API_KEY_INVALID") {
		t.Errorf("Expected Gemini error, got: %v", err)
	}
}
