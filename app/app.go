package app

import (
	"context"
	"log"
	"os"
	"rag-app/chat"
	"rag-app/config"
	"rag-app/ingest"
	"rag-app/llm"
	"rag-app/rag"
	"rag-app/vector"
	"rag-app/vector/pgvector"
	"rag-app/web"
	"sync"
)

func Run(parent context.Context, cfg config.Config) error {
	logger := log.New(os.Stderr, "[rag]", log.LstdFlags)

	ctx, cancel := context.WithCancel(parent)

	defer cancel()

	client := llm.New(cfg)

	embedder := llm.NewEmbedder(cfg)

	store, err := openStore(ctx, cfg)

	if err != nil {
		logger.Printf("vector store disabled: %v", err)
	}

	var wg sync.WaitGroup

	if store != nil {
		wg.Go(func() {
			opts := ingest.Options{
				SourceDir:    cfg.IngestDir,
				ProcessedDir: cfg.ProcessedDir,
			}

			if err := ingest.Watch(ctx, opts, embedder, store, logger); err != nil && ctx.Err() == nil {
				logger.Printf("watcher stopped: %v", err)
			}
		})
	}

	logger.Printf("watching %s for new documents", cfg.IngestDir)

	if store != nil {
		defer store.Close()
		logger.Printf("vector store ready")
	}

	var retriever *rag.Retriever

	if store != nil {
		retriever = rag.New(embedder, store, rag.Options{
			TopK:     5,
			Rewriter: rag.NewRewriter(client),
		})
	}

	if cfg.HTTPAddr != "" {
		srv, err := web.New(client, embedder, retriever, web.Options{
			Addr:             cfg.HTTPAddr,
			SystemPromptFile: cfg.SystemPromptFile,
			Store:            store,
			ProcessedDir:     cfg.ProcessedDir,
			ImagesDir:        cfg.ImagesDir,
		})

		if err != nil {
			logger.Printf("web server disabled %v", err)
		} else {

			wg.Go(func() {
				if err := srv.Run(ctx, cfg.HTTPAddr); err != nil && ctx.Err() == nil {
					logger.Printf("webserver stopped %v", err)
				}
			})

			logger.Printf("web chat at http://localhost%s/chat", cfg.HTTPAddr)
		}

	}

	replErr := chat.RunREPL(ctx, client, retriever, chat.Options{
		SystemPromptFile: cfg.SystemPromptFile,
	})

	cancel()
	wg.Wait()
	return replErr
}

func openStore(ctx context.Context, cfg config.Config) (vector.Store, error) {

	if cfg.DatabaseURL == "" {
		return nil, nil
	}

	store, err := pgvector.New(ctx, pgvector.Options{
		DSN:          cfg.DatabaseURL,
		EmbeddingDim: cfg.EmbeddingDim,
	})

	if err != nil {
		return nil, err
	}

	return store, nil
}
