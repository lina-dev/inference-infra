package vllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(addr string) *Client {
	return &Client{
		baseURL: "http://" + addr,
		http:    &http.Client{},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

const summaryPrompt = `You are a precise summarization assistant. Given the following audio transcript with timestamps, produce a concise summary covering the key topics discussed. Transcript:\n\n%s`

// Summarize calls the vLLM OpenAI-compatible /v1/chat/completions endpoint.
func (c *Client) Summarize(ctx context.Context, transcript string) (string, error) {
	req := chatRequest{
		Model: "default",
		Messages: []message{
			{Role: "system", Content: "You are a helpful assistant that summarizes audio transcripts."},
			{Role: "user", Content: fmt.Sprintf(summaryPrompt, transcript)},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("vllm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vllm status %d: %s", resp.StatusCode, b)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode vllm response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty vllm response")
	}
	return chatResp.Choices[0].Message.Content, nil
}
