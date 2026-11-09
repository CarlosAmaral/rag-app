package rag

import (
	"context"
	"fmt"
	"github.com/CarlosAmaral/rag-app/llm"
	"strings"
)

type Rewriter struct {
	client *llm.Client
}

const rewriteSystemPrompt = `You rewrite the user's latest message into a standalone search query
Given the conversation, output a single search query that:
- Captures the topic and intent of the latest user message.
- Resolves pronouns and references using prior turns ("it", "they", "that one").
- Stays concise - keywords and short phrases, not full sentences.

If the later user message already stands on its own with no references to prior turns, output it verbatim.

Output only the query. No preamble, no quotes, no explanation.
`

func (r *Rewriter) Rewrite(ctx context.Context, history []llm.Message) (string, error) {
	last := lastUserMessage(history)

	if last == "" {
		return "", nil
	}

	if !hasAssistantTurn(history) {
		return last, nil
	}

	msgs := []llm.Message{
		{
			Role:    "system",
			Content: rewriteSystemPrompt,
		},
		{
			Role:    "user",
			Content: formatConversation(history),
		},
	}

	reply, err := r.client.ChatStream(ctx, msgs, nil)

	if err != nil {
		return "", fmt.Errorf("rewrite call: %w", err)
	}

	out := strings.TrimSpace(reply.Content)
	out = strings.Trim(out, `"'`)

	if out == "" {
		return last, nil
	}

	return out, nil

}

func NewRewriter(client *llm.Client) *Rewriter {
	return &Rewriter{client: client}
}

func formatConversation(history []llm.Message) string {
	var sb strings.Builder
	sb.WriteString("Conversation so far:\n\n")

	for _, m := range history {

		switch m.Role {
		case "user":
			sb.WriteString("User: ")
		case "assistance":
			sb.WriteString("Assistant: ")

		default:
			continue
		}

		sb.WriteString(m.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Rewrite the user's latest message as a standalone search query.")
	return sb.String()
}

func hasAssistantTurn(history []llm.Message) bool {
	for _, m := range history {
		if m.Role == "assistant" {
			return true
		}
	}

	return false
}
