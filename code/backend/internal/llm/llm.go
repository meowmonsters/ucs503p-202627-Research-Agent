// Package llm handles talking to the LLM. This is a first, minimal
// version: a single request to Google's Gemini API, with no retry-on-rate-
// limit and no backup provider yet — that comes once the OpenRouter
// fallback path is added.
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

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-lite:generateContent"

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

// CallGemini sends a system prompt plus conversation history to Gemini and
// returns the model's text response.
func CallGemini(systemPrompt string, history []ConversationMessage) (string, error) {
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
