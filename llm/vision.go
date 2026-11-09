package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/v3"
)

const captionPrompt = "Describe this image into 2-2 sentences for a search index - what it is, key details, style. Do not interpret or speculate; describe only what's shown."

func (c *Client) HasVision() bool {
	return c.cfg.VisionModel != ""
}

func (c *Client) DescribeImg(ctx context.Context, mime string, image []byte) (string, error) {

	if c.cfg.VisionModel == "" {
		return "", errors.New("no vision model configured")
	}

	if len(image) == 0 {
		return "", errors.New("empty image")
	}

	if mime == "" {
		mime = http.DetectContentType(image)
	}

	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)

	resp, err := c.sdk.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: c.cfg.VisionModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: dataURL,
				}),
			}),
		},
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices found")
	}

	return resp.Choices[0].Message.Content, nil
}
