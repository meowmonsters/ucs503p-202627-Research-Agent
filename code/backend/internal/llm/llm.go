// Package llm handles talking to the LLM: a direct call to Google's
// Gemini API, with an OpenRouter-routed backup path used if Gemini's call
// fails and an OpenRouter key is configured.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	geminiAPIURL     = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-lite:generateContent"
	openRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"
	openRouterModel  = "google/gemini-2.0-flash-001"
)

// ConversationMessage is one turn in the conversation sent to the model.
// Role is "user" or "model".
type ConversationMessage struct {
	Role string
	Text string
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction struct {
		Parts []geminiPart `json:"parts"`
	} `json:"systemInstruction"`
	Contents         []geminiContent `json:"contents"`
	GenerationConfig struct {
		Temperature     float64 `json:"temperature"`
		MaxOutputTokens int     `json:"maxOutputTokens"`
		TopP            float64 `json:"topP"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func callDirectGemini(systemPrompt string, history []ConversationMessage) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	var body geminiRequest
	body.SystemInstruction.Parts = []geminiPart{{Text: systemPrompt}}
	for _, msg := range history {
		role := "user"
		if msg.Role == "model" {
			role = "model"
		}
		body.Contents = append(body.Contents, geminiContent{Role: role, Parts: []geminiPart{{Text: msg.Text}}})
	}
	body.GenerationConfig.Temperature = 0.4
	body.GenerationConfig.MaxOutputTokens = 2048
	body.GenerationConfig.TopP = 0.9

	reqBytes, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, geminiAPIURL+"?key="+apiKey, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	var sb strings.Builder
	if len(parsed.Candidates) > 0 {
		for _, p := range parsed.Candidates[0].Content.Parts {
			sb.WriteString(p.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf("Gemini API returned an empty response")
	}
	return text, nil
}

// --- OpenRouter (backup path) ---

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterRequest struct {
	Model       string              `json:"model"`
	Messages    []openRouterMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callOpenRouter(systemPrompt string, history []ConversationMessage) (string, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}

	messages := make([]openRouterMessage, 0, len(history)+1)
	messages = append(messages, openRouterMessage{Role: "system", Content: systemPrompt})
	for _, msg := range history {
		role := "user"
		if msg.Role == "model" {
			role = "assistant"
		}
		messages = append(messages, openRouterMessage{Role: role, Content: msg.Text})
	}

	reqBody, err := json.Marshal(openRouterRequest{
		Model:       openRouterModel,
		Messages:    messages,
		Temperature: 0.4,
		MaxTokens:   2048,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, openRouterAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenRouter API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var parsed openRouterResponse
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse OpenRouter response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter returned an empty response")
	}
	text := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("OpenRouter returned an empty response")
	}
	return text, nil
}

// CallGemini sends a system prompt plus conversation history to Gemini and
// returns the model's text response. If the direct Gemini call fails and
// OPENROUTER_API_KEY is configured, it retries once through OpenRouter
// before giving up.
func CallGemini(systemPrompt string, history []ConversationMessage) (string, error) {
	text, err := callDirectGemini(systemPrompt, history)
	if err == nil {
		return text, nil
	}

	if os.Getenv("OPENROUTER_API_KEY") == "" {
		return "", err
	}

	text, orErr := callOpenRouter(systemPrompt, history)
	if orErr != nil {
		// Neither provider worked — surface the original Gemini error,
		// since that's the primary path.
		return "", err
	}
	return text, nil
}
