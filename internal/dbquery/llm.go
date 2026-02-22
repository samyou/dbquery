package dbquery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

var codeFencePattern = regexp.MustCompile("(?s)^```(?:\\w+)?\\s*(.*?)\\s*```$")

func generateSQL(ctx context.Context, cfg Config, schemaContext, naturalQuery string) (string, error) {
	endpoint := strings.TrimRight(cfg.LLMBaseURL, "/") + "/chat/completions"

	modeLine := "Generate one read-only SQL query."
	if cfg.AllowWrite {
		modeLine = "Generate one SQL query matching the request."
	}

	systemPrompt := strings.Join([]string{
		"You are a senior SQL engineer.",
		"Translate user requests into valid SQL for the specified dialect.",
		modeLine,
		"Use only schema shown in the context.",
		"Return only raw SQL. No markdown, no explanation, no backticks.",
		fmt.Sprintf("Target dialect: %s.", cfg.DBType),
		fmt.Sprintf("Target row limit: %d unless user asks for another limit.", cfg.Limit),
	}, "\n")

	userPrompt := fmt.Sprintf(
		"User request:\n%s\n\nSchema context:\n%s\n",
		naturalQuery,
		schemaContext,
	)

	payload := chatCompletionRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("LLM request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}

	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("LLM response has no choices")
	}

	return decoded.Choices[0].Message.Content, nil
}

func normalizeSQL(sqlQuery string) string {
	q := strings.TrimSpace(sqlQuery)
	if m := codeFencePattern.FindStringSubmatch(q); len(m) == 2 {
		q = strings.TrimSpace(m[1])
	}

	if strings.HasPrefix(strings.ToLower(q), "sql:") {
		q = strings.TrimSpace(q[4:])
	}

	q = strings.TrimSpace(q)
	return q
}
