package rag

import (
	"fmt"
	"rag-app/vector"
	"strings"
)

const contextPreample = `Use the following excerpts from the document collection to answer the question.
Cite sources from the file name when you draw them. If the excerpts do not answer the question, say so before answering
from general knowledge.`

const unknownSource = "(unknown source)"

func formatContext(hits []vector.Result) string {

	if len(hits) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(contextPreample)
	sb.WriteString("\n\n--- Excertps ---\n\n")

	for i, h := range hits {
		source := h.Metadata["source"]
		if source == "" {
			source = unknownSource
		}

		if h.Metadata["type"] == "image" && h.Metadata["image_path"] != "" {
			fmt.Fprintf(&sb, "[%d] Source: %s [image: %s] (similarity %.2f\n%s\n\n)", i+1, source, h.Metadata["image_path"], h.Score, h.Content)
		}

		fmt.Fprintf(&sb, "[%d] Source: %s (similarity %.2f)\n%s\n", i+1, source, h.Score, h.Content)
	}

	return strings.TrimSpace(sb.String())
}
