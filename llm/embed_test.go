package llm

import (
	"context"
	"github.com/CarlosAmaral/rag-app/config"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestEmbedPreservesValuesAndInputOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"object": "embedding", "index": 1, "embedding": [-0.75, 0, 0.125]},
				{"object": "embedding", "index": 0, "embedding": [0.25, -0.5, 1]}
			],
			"model": "test-embedding",
			"usage": {"prompt_tokens": 2, "total_tokens": 2}
		}`))
	}))
	defer server.Close()

	client := NewEmbedder(config.Config{
		EmbeddingBaseURL: server.URL + "/v1/",
		EmbeddingAPIKey:  "test-key",
		EmbeddingModel:   "test-embedding",
	})
	got, err := client.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	want := [][]float32{{0.25, -0.5, 1}, {-0.75, 0, 0.125}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Embed = %v, want %v", got, want)
	}
}
