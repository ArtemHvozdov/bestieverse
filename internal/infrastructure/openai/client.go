package openai

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// Client wraps the OpenAI SDK for image generation.
type Client struct {
	client *openai.Client
	model  string
}

// NewClient creates a new OpenAI client with the given API key and model.
func NewClient(apiKey, model string) *Client {
	return &Client{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

// GenerateCollage sends a prompt to OpenAI and returns the generated image as PNG bytes.
func (c *Client) GenerateCollage(ctx context.Context, prompt string) ([]byte, error) {
	resp, err := c.client.CreateImage(ctx, openai.ImageRequest{
		Model:          c.model,
		Prompt:         prompt,
		N:              1,
		Size:           openai.CreateImageSize1024x1024,
		ResponseFormat: openai.CreateImageResponseFormatB64JSON,
	})
	if err != nil {
		return nil, fmt.Errorf("openai.GenerateCollage: create image: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai.GenerateCollage: empty response")
	}
	imageBytes, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("openai.GenerateCollage: decode base64: %w", err)
	}
	return imageBytes, nil
}
