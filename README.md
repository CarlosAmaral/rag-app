# RAG App

A Go application for retrieval-augmented chat with a terminal interface and an optional browser interface. It streams responses from an OpenAI-compatible API, indexes text documents and image descriptions, and retrieves related chunks from PostgreSQL with pgvector.

## Project packages

The module is `rag-app`. It contains ten Go packages, including the executable package.

| Import path | Directory | Responsibility |
| --- | --- | --- |
| `rag-app/cmd/rag` (`main`) | [cmd/rag](cmd/rag/main.go) | Executable entry point and signal handling. |
| `rag-app/app` | [app](app/app.go) | Application lifecycle and dependency wiring. |
| `rag-app/config` | [config](config/config.go) | Environment configuration and defaults. |
| `rag-app/chat` | [chat](chat/repl.go) | Interactive terminal chat. |
| `rag-app/ingest` | [ingest](ingest) | Document watching, chunking, and indexing. |
| `rag-app/llm` | [llm](llm) | Chat, embedding, and vision API clients. |
| `rag-app/rag` | [rag](rag) | Query rewriting, retrieval, and context formatting. |
| `rag-app/vector` | [vector](vector/store.go) | Shared vector storage interface and data types. |
| `rag-app/vector/pgvector` | [vector/pgvector](vector/pgvector/pgvector.go) | PostgreSQL implementation of vector storage. |
| `rag-app/web` | [web](web) | Browser chat, streaming API, uploads, and request filtering. |

### `cmd/rag`

`main` creates a context canceled by an interrupt or SIGTERM, loads configuration through `config.Load`, and calls `app.Run`. Application errors cause a nonzero exit.

### `app`

`Run` builds the chat and embedding clients, opens the optional vector store, starts the ingestion watcher, creates the retriever, and starts HTTP serving when configured. It runs the terminal REPL in the foreground and cancels background services when the REPL exits.

A missing database URL disables storage and retrieval. A store initialization failure is logged, and chat can continue without retrieval. The package closes the store and waits for managed background services during shutdown.

### `config`

`Config` holds model endpoints, credentials, model names, database settings, filesystem paths, and the HTTP address. `Load` reads `.env` using `godotenv`, reads environment variables, and applies defaults. Existing environment values take precedence over `.env` values.

### `chat`

`RunREPL` reads terminal input, maintains conversation history, optionally seeds it from a system prompt file, and streams model output with a spinner while waiting. It retrieves document context when a retriever is available and adds that context to the current request without storing it in conversation history.

Use `Q`, `quit`, or `/exit` to quit. End-of-input also ends the session. Retrieval failures are reported while allowing ordinary chat to continue.

### `ingest`

- `Watch` scans the source directory on startup and watches for create/write events with a 500 ms debounce. Watching is nonrecursive. Successfully indexed files move to the processed directory.
- `ProcessContent` accepts `.txt`, `.md`, and `.markdown` files, splits their text, generates embeddings, and replaces stored chunks for the source filename.
- `ProcessImage` indexes a supplied description for `.png`, `.jpeg`, `.jpg`, `.webp`, or `.gif` files. It records an `/images/` path in metadata; it does not generate captions or save image bytes itself.
- `IsSupported` and `IsImage` expose supported-extension checks.

`Options` controls source/processed directories and chunk sizing. Defaults are 1,000 bytes per chunk with 100 bytes of overlap. Chunking prefers paragraph, sentence, or word boundaries near the end of a chunk. Stored metadata includes the source filename, chunk index, chunk count, and ingestion timestamp. Chunk IDs use `<filename>#<index>`.

### `llm`

`Client` wraps the OpenAI Go SDK for compatible API endpoints. `New` configures the chat client; `NewEmbedder` configures a separate embedding client.

- `Message` represents a conversation message.
- `ChatStream` calls streaming chat completions, forwards text deltas to an optional callback, and returns the accumulated reply.
- `Embedder` is the interface used by ingestion and retrieval. `Client.Embed` requests embeddings, restores input order using response indices, and converts vector values to `float32`.
- `HasVision` reports whether a vision model is configured. `DescribeImg` sends image bytes as a base64 data URL to that model and returns its response text.

[embed_test.go](llm/embed_test.go) tests embedding values and ordering with a mock HTTP server.

### `rag`

`New` creates a `Retriever` from an embedder, vector store, and `Options`. `Retrieve` builds a search query, embeds it, retrieves similar chunks, and formats excerpts with filenames and similarity scores for the chat request. The default result count is five.

`NewRewriter` creates a model-backed `Rewriter` intended to turn follow-up messages into standalone search queries. Retrieval falls back to the latest user message if rewriting fails or returns no query. The first user turn skips the rewrite API call.

Context formatting asks the answering model to cite filenames and acknowledge when excerpts do not answer the question.

### `vector`

Defines the storage contract independently of PostgreSQL:

- `Document`: ID, content, string metadata, and embedding.
- `Result`: a document plus its similarity score.
- `Store`: `Upsert`, `Query`, `Delete`, `DeleteBySource`, and `Close`.

Ingestion and retrieval depend on this interface, allowing another storage backend to be implemented without changing their storage contract.

### `vector/pgvector`

`New` accepts a database DSN and embedding dimension, enables the `vector` extension, registers pgvector types with pgx, opens a connection pool, and creates the `documents` table and HNSW cosine index if absent.

`Store` implements `vector.Store`. Upserts run in a transaction, metadata is stored as JSONB, and queries order by cosine distance and return similarity as `1 - distance`. Deletion supports IDs or the source filename in metadata.

The configured dimension must match the embedding model output. Initialization does not resize an existing table's vector column.

### `web`

`New` builds a server using embedded templates from [web/templates](web/templates). `Routes` exposes its HTTP handler; `Run` starts HTTP serving and handles context-driven shutdown.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/chat` | Browser chat interface. |
| POST | `/api/chat/stream` | JSON conversation input; server-sent events named `delta`, `error`, and `done`. |
| POST | `/api/upload` | Multipart text/Markdown upload using the `file` field; indexes and saves the document. |
| POST | `/api/upload/image` | Multipart `image` and required `description` fields; saves the image and indexes its description. Registered when an image directory is configured. |
| GET | `/images/*` | Serves files from the configured image directory. |
| POST | `/api/caption` | Generates image description text. Registered only when a vision model is configured. |

Document and image indexing require a vector store. Chat can operate without one.

`InjectionDefense` filters the chat and upload routes using regular expressions for instruction overrides, role takeover, prompt disclosure, safety bypass requests, and role-tag smuggling. It inspects user messages in JSON, multipart text fields, and supported text-file parts. Each scanned value/file is limited to its first 64 KiB. This is a heuristic filter; terminal chat and filesystem ingestion do not pass through it. Captioning is outside the middleware group.

The middleware caps JSON and multipart request bodies at 1 MiB. Upload handlers separately specify 10 MiB, but requests passing through the middleware encounter its lower limit first. The caption handler uses the 10 MiB limit. `web/file.go` currently contains only the package declaration.

## How the packages work together

1. `cmd/rag` loads `config` and starts `app`.
2. `app` constructs `llm` clients and the optional `vector/pgvector` store.
3. `ingest` splits documents or image descriptions, embeds them through `llm`, and stores chunks through `vector.Store`.
4. `chat` or `web` passes conversation history to `rag`, which optionally rewrites the query, embeds it, searches the store, and formats context.
5. `llm` streams the answer back to the terminal or browser.

## Running the application

Use a Go toolchain compatible with the `go 1.27.1` directive in [go.mod](go.mod), a chat-completions-compatible API, and an embeddings-compatible API for retrieval. PostgreSQL with pgvector is needed for indexing and retrieval; Docker Compose is provided for local database setup.

Start the database when using retrieval:

```sh
docker compose up -d postgres
```

Create or edit `.env` with settings for your providers. This example assumes an embedding endpoint serving `nomic-embed-text` on port 11434; Compose starts only PostgreSQL, so the model service must be provided separately.

```dotenv
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=replace-with-your-key
OPENAI_MODEL=gpt-4o-mini
EMBEDDING_BASE_URL=http://localhost:11434/v1
EMBEDDING_MODEL=nomic-embed-text
EMBEDDING_DIM=768
DATABASE_URL=postgres://rag:rag@localhost:5432/rag?sslmode=disable
HTTP_ADDR=:8080
SYSTEM_PROMPT_FILE=./prompts/system-custom.md
```

Set `EMBEDDING_API_KEY` if the embedding endpoint requires authentication. Then run from the repository root:

```sh
go run ./cmd/rag
```

The terminal starts a chat session. With `HTTP_ADDR=:8080`, the browser interface is at `http://localhost:8080/chat`. Keep terminal input open while using the browser: leaving the REPL also shuts down HTTP serving.

Place supported text documents directly in `documents/` for automatic indexing, or upload through the browser. Images use the browser upload flow with a description. Vision captioning is optional and requires `VISION_MODEL` on the chat endpoint.

## Configuration reference

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | Chat and vision API base URL. |
| `OPENAI_API_KEY` | Empty | Chat and vision API key. |
| `OPENAI_MODEL` | `gpt-4o-mini` | Chat and query-rewriting model. |
| `SYSTEM_PROMPT_FILE` | Empty | Optional system prompt file. |
| `DATABASE_URL` | Empty | PostgreSQL DSN; empty disables the vector store. |
| `EMBEDDING_DIM` | `768` | Vector column dimension for table creation. |
| `EMBEDDING_BASE_URL` | Chat base URL | Embedding API base URL. |
| `EMBEDDING_API_KEY` | Conditional fallback | Falls back to the chat API key only when `EMBEDDING_BASE_URL` is unset. |
| `EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model; must be available at the configured endpoint. |
| `INGEST_DIR` | `./documents` | Directory watched for text documents. |
| `PROCESSED_DIR` | `./documents/processed` | Destination for indexed documents. |
| `HTTP_ADDR` | Empty | HTTP listen address; empty disables the web server. |
| `IMAGES_DIR` | `./documents/images` | Uploaded image storage directory. |
| `VISION_MODEL` | Empty | Optional image-captioning model. |

## External Go dependencies

These are the libraries imported directly by application code. Although currently marked `indirect` in `go.mod`, they serve the following roles:

| Module | Used for |
| --- | --- |
| `github.com/openai/openai-go/v3` | Chat completions, embeddings, and vision requests. |
| `github.com/joho/godotenv` | Loading `.env` configuration. |
| `github.com/fsnotify/fsnotify` | Filesystem notifications for ingestion. |
| `github.com/go-chi/chi/v5` | HTTP routing and panic recovery middleware. |
| `github.com/jackc/pgx/v5` | PostgreSQL connections, pooling, SQL, and transactions. |
| `github.com/pgvector/pgvector-go` | PostgreSQL vector values. |
| `github.com/pgvector/pgvector-go/pgx` | pgvector type registration for pgx. |

Other modules listed in `go.mod` support these libraries: `coder/websocket`; `jackc/pgpassfile`, `pgservicefile`, and `puddle/v2`; `tidwall/gjson`, `match`, `pretty`, and `sjson`; `x448/float16`; and `golang.org/x/sync`, `x/sys`, and `x/text`. Exact versions are recorded in `go.mod` and checksums in `go.sum`.

## Supporting files and verification

- [docker-compose.yml](docker-compose.yml) runs PostgreSQL with pgvector, local `rag` credentials, port 5432, and a persistent volume.
- [create-table.sql](create-table.sql) provides a manual schema with 768-dimensional vectors. The store also initializes its schema automatically, so manual execution is optional.
- [prompts/system-custom.md](prompts/system-custom.md) is an available system prompt, loaded only when configured.
- `documents/` holds ingestion input, processed documents, and uploaded images; it is not a Go package.

Run the existing test suite with:

```sh
go test ./...
```
