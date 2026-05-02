package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ai-soft-skills-trainer/internal/models"
)

type Client struct {
	serviceURL  string
	modelName   string
	temperature float64
	maxTokens   int
	topP        float64
	httpClient  *http.Client
}

type inferenceGenerateRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Temperature float64  `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type inferenceChoice struct {
	Text    string `json:"text"`
	Message struct {
		Content string `json:"content"`
	} `json:"message,omitempty"`
}

type inferenceGenerateResponse struct {
	Choices []inferenceChoice `json:"choices"`
}

type generationPayload struct {
	SystemPrompt string           `json:"system_prompt"`
	Messages     []models.Message `json:"messages"`
}

func NewClient(serviceURL, modelName string, maxTokens int, temperature float64, topP float64) *Client {
	if serviceURL == "" {
		serviceURL = "http://localhost:8087"
	}
	if modelName == "" {
		modelName = "GigaChat"
	}
	if maxTokens <= 0 {
		maxTokens = 120
	}
	if temperature <= 0 {
		temperature = 0.55
	}
	if topP <= 0 {
		topP = 0.85
	}

	return &Client{
		serviceURL:  strings.TrimRight(serviceURL, "/"),
		modelName:   modelName,
		temperature: temperature,
		maxTokens:   maxTokens,
		topP:        topP,
		httpClient:  &http.Client{Timeout: 180 * time.Second},
	}
}

func (c *Client) SendMessage(ctx context.Context, systemPrompt string, history []models.Message) (string, error) {
	prompt := c.buildPrompt(systemPrompt, history)

	body, err := json.Marshal(inferenceGenerateRequest{
		Model:       c.modelName,
		Prompt:      prompt,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		TopP:        c.topP,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.generateURL(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("inference request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("inference error: %s: %s", resp.Status, strings.TrimSpace(string(errorBody)))
	}

	var generateResp inferenceGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&generateResp); err != nil {
		return "", err
	}
	if len(generateResp.Choices) == 0 {
		return "", errors.New("inference returned no choices")
	}

	if generateResp.Choices[0].Message.Content != "" {
		return strings.TrimSpace(generateResp.Choices[0].Message.Content), nil
	}
	return strings.TrimSpace(generateResp.Choices[0].Text), nil
}

func (c *Client) GetModels(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serviceURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inference models request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("inference models error: %s: %s", resp.Status, strings.TrimSpace(string(errorBody)))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) generateURL() string {
	return c.serviceURL + "/v1/completions"
}

func (c *Client) buildPrompt(systemPrompt string, history []models.Message) string {
	if len(history) == 0 {
		return systemPrompt
	}

	var builder strings.Builder
	builder.WriteString(systemPrompt)
	builder.WriteString("\n\n")
	for _, message := range history {
		role := strings.Title(strings.ToLower(message.Role))
		if role == "Assistant" {
			role = "Assistant"
		} else if role == "User" {
			role = "User"
		}
		builder.WriteString(role)
		builder.WriteString(": ")
		builder.WriteString(message.Text)
		builder.WriteString("\n")
	}
	builder.WriteString("Assistant:")
	return builder.String()
}
