package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"paperMC_backend/internal/database"
)

var (
	reIPv4     = regexp.MustCompile(`\b(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)
	reUUID     = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reSecrets  = regexp.MustCompile(`(?i)(password|rcon\.password|token|secret|api[_-]?key)\s*[:=]\s*([^\s,;]+)`)
	reUnixHome = regexp.MustCompile(`/(?:home|Users)/[a-zA-Z0-9_\-\.]+`)
	reWinHome  = regexp.MustCompile(`[A-Za-z]:\\(?:Users|Documents and Settings)\\[a-zA-Z0-9_\-\.]+`)
)

// SanitizeLog scrubs local filesystem paths, passwords/secrets, IP addresses, and player UUIDs.
func SanitizeLog(rawLog string, workDir string) string {
	if rawLog == "" {
		return ""
	}

	sanitized := rawLog

	// 1. Scrub explicit server working directory
	if workDir != "" {
		sanitized = strings.ReplaceAll(sanitized, workDir, "[SERVER_DIR]")
	}

	// 2. Scrub home directory paths
	sanitized = reUnixHome.ReplaceAllString(sanitized, "/home/[USER]")
	sanitized = reWinHome.ReplaceAllString(sanitized, `C:\Users\[USER]`)

	// 3. Scrub secrets / passwords
	sanitized = reSecrets.ReplaceAllString(sanitized, "${1}=[REDACTED]")

	// 4. Scrub UUIDs
	sanitized = reUUID.ReplaceAllString(sanitized, "[UUID_REDACTED]")

	// 5. Scrub IP addresses (avoid 127.0.0.1 or 0.0.0.0)
	sanitized = reIPv4.ReplaceAllStringFunc(sanitized, func(ip string) string {
		if ip == "127.0.0.1" || ip == "0.0.0.0" {
			return ip
		}
		return "[IP_REDACTED]"
	})

	return sanitized
}

// Client represents the AI diagnostic client.
type Client struct {
	httpClient *http.Client
}

// NewClient initializes a new AI client with a default timeout.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// SystemPrompt defines the persona and formatting instructions for LLMs.
const SystemPrompt = `You are an expert Minecraft server administrator and Java runtime diagnostics engineer.
Analyze the provided Minecraft server crash report, diagnostic classification, and sanitized error logs.
Explain the root cause clearly in simple terms, identify the likely culprit, and provide actionable, step-by-step instructions to fix it.
Format your response in clean Markdown with headers, bullet points, and code snippets if config edits are needed.`

// ExplainCrash submits a crash report to OpenAI-compatible or Google Gemini endpoints.
func (c *Client) ExplainCrash(ctx context.Context, settings *database.AISettings, report *database.CrashReport, customPrompt string) (string, error) {
	if settings == nil || !settings.IsEnabled {
		return "", errors.New("AI diagnostics is disabled in server settings")
	}

	provider := strings.ToLower(strings.TrimSpace(settings.Provider))
	apiKey := strings.TrimSpace(settings.APIKey)

	// Ollama or local endpoints might not require an API key
	isLocal := strings.Contains(strings.ToLower(settings.BaseURL), "localhost") || strings.Contains(strings.ToLower(settings.BaseURL), "127.0.0.1")
	if apiKey == "" && !isLocal && provider != "ollama" {
		return "", errors.New("API key is required for AI diagnostics")
	}

	sanitizedLog := SanitizeLog(report.RawLog, "")
	// Truncate to last 4000 characters if excessive
	if len(sanitizedLog) > 4000 {
		sanitizedLog = "...[truncated]...\n" + sanitizedLog[len(sanitizedLog)-4000:]
	}

	userContent := fmt.Sprintf(`### Crash Information
- **Category:** %s
- **Title:** %s
- **Culprit:** %s
- **Local Heuristic Summary:** %s
- **Recommendation:** %s

### Sanitized Crash / Log Dump:
`+"```"+`
%s
`+"```",
		report.Category,
		report.Title,
		report.Culprit,
		report.Summary,
		report.Recommendation,
		sanitizedLog,
	)

	if strings.TrimSpace(customPrompt) != "" {
		userContent += fmt.Sprintf("\n\n### User Question / Context:\n%s", strings.TrimSpace(customPrompt))
	}

	switch provider {
	case "gemini":
		return c.queryGemini(ctx, settings, apiKey, userContent)
	case "openai", "ollama", "groq", "deepseek", "":
		return c.queryOpenAI(ctx, settings, apiKey, userContent)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", provider)
	}
}

// OpenAIChatRequest payload for OpenAI-compatible completions
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *Client) queryOpenAI(ctx context.Context, settings *database.AISettings, apiKey string, userContent string) (string, error) {
	url := strings.TrimSpace(settings.BaseURL)
	if url == "" {
		url = "https://api.openai.com/v1/chat/completions"
	} else if !strings.HasSuffix(url, "/chat/completions") {
		url = strings.TrimRight(url, "/") + "/chat/completions"
	}

	model := strings.TrimSpace(settings.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}

	reqPayload := OpenAIChatRequest{
		Model: model,
		Messages: []OpenAIMessage{
			{Role: "system", Content: SystemPrompt},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.3,
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read AI response body: %w", err)
	}

	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("AI API returned HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return "", fmt.Errorf("failed to parse AI response: %w", err)
	}

	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return "", fmt.Errorf("AI API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("AI API returned HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return "", errors.New("AI API returned an empty completion")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// Gemini Request / Response models
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

func (c *Client) queryGemini(ctx context.Context, settings *database.AISettings, apiKey string, userContent string) (string, error) {
	model := strings.TrimSpace(settings.Model)
	if model == "" {
		model = "gemini-1.5-flash"
	}

	url := strings.TrimSpace(settings.BaseURL)
	if url == "" {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	} else {
		if !strings.Contains(url, "key=") && apiKey != "" {
			separator := "?"
			if strings.Contains(url, "?") {
				separator = "&"
			}
			url = fmt.Sprintf("%s%skey=%s", url, separator, apiKey)
		}
	}

	fullPrompt := fmt.Sprintf("%s\n\n%s", SystemPrompt, userContent)
	reqPayload := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: fullPrompt},
				},
			},
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Gemini response body: %w", err)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("Gemini API returned HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return "", fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if geminiResp.Error != nil && geminiResp.Error.Message != "" {
		return "", fmt.Errorf("Gemini API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 ||
		geminiResp.Candidates[0].Content.Parts[0].Text == "" {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("Gemini API returned HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return "", errors.New("Gemini API returned an empty response")
	}

	return strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text), nil
}
