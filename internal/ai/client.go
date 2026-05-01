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
	tritonURL  string
	modelName  string
	httpClient *http.Client
}

type tritonInput struct {
	Name     string   `json:"name"`
	Shape    []int    `json:"shape"`
	Datatype string   `json:"datatype"`
	Data     []string `json:"data"`
}

type tritonInferRequest struct {
	Inputs []tritonInput `json:"inputs"`
}

type tritonOutput struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

type tritonInferResponse struct {
	Outputs []tritonOutput `json:"outputs"`
}

type generationPayload struct {
	SystemPrompt string           `json:"system_prompt"`
	Messages     []models.Message `json:"messages"`
}

func NewClient(tritonURL, modelName string) *Client {
	if tritonURL == "" {
		tritonURL = "http://localhost:8000"
	}
	if modelName == "" {
		modelName = "softskill_generator"
	}

	return &Client{
		tritonURL:  strings.TrimRight(tritonURL, "/"),
		modelName:  modelName,
		httpClient: &http.Client{Timeout: 180 * time.Second},
	}
}

func (c *Client) SendMessage(ctx context.Context, systemPrompt string, history []models.Message) (string, error) {
	payload, err := json.Marshal(generationPayload{
		SystemPrompt: systemPrompt,
		Messages:     history,
	})
	if err != nil {
		return "", err
	}

	reqBody := tritonInferRequest{
		Inputs: []tritonInput{
			{
				Name:     "PROMPT",
				Shape:    []int{1},
				Datatype: "BYTES",
				Data:     []string{string(payload)},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.inferURL(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("triton inference request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("triton inference error: %s: %s", resp.Status, strings.TrimSpace(string(errorBody)))
	}

	var inferResp tritonInferResponse
	if err := json.NewDecoder(resp.Body).Decode(&inferResp); err != nil {
		return "", err
	}

	for _, output := range inferResp.Outputs {
		if output.Name != "TEXT" {
			continue
		}

		var values []string
		if err := json.Unmarshal(output.Data, &values); err != nil {
			return "", err
		}
		if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
			return "", errors.New("triton returned an empty generation")
		}
		return strings.TrimSpace(values[0]), nil
	}

	return "", errors.New("triton response has no TEXT output")
}

func (c *Client) GetModels(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.tritonURL+"/v2/models/"+c.modelName, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("triton model metadata request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("triton model metadata error: %s: %s", resp.Status, strings.TrimSpace(string(errorBody)))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) inferURL() string {
	return c.tritonURL + "/v2/models/" + c.modelName + "/infer"
}
