.DEFAULT_GOAL := help
SHELL := /bin/bash

# ---------------------------------------------------------------------------
# Local llama.cpp embedding/rerank servers + bench plumbing
# ---------------------------------------------------------------------------

# Override on the command line, e.g.
#   make llama-embed MODELS_DIR=/srv/models PORT_EMBED=9001
MODELS_DIR     ?= $(HOME)/mem-bench-deps/models
LOG_DIR        ?= $(HOME)/mem-bench-deps
EMBED_MODEL    ?= all-MiniLM-L6-v2-Q8_0.gguf
RERANK_MODEL   ?= bge-reranker-v2-m3-Q4_K_M.gguf
PORT_EMBED     ?= 8092
PORT_RERANK    ?= 8093
HOST           ?= 127.0.0.1

EMBED_PID  := $(LOG_DIR)/llama-server-embed.pid
EMBED_LOG  := $(LOG_DIR)/llama-server-embed.log
RERANK_PID := $(LOG_DIR)/llama-server-rerank.pid
RERANK_LOG := $(LOG_DIR)/llama-server-rerank.log

.PHONY: help
help: ## list targets
	@awk 'BEGIN{FS=":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

# ---------------------------------------------------------------------------
# Build / test
# ---------------------------------------------------------------------------

.PHONY: build
build: ## build the mem CLI to /tmp/mem-test
	go build -o /tmp/mem-test ./cmd/mem/

.PHONY: test
test: ## go test -race -shuffle=on across the repo
	go test -race -shuffle=on -count=1 ./...

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: bench-build
bench-build: ## build all three bench binaries to /tmp
	go build -o /tmp/longmemeval-bench ./benchmarks/longmemeval/
	go build -o /tmp/locomo-bench ./benchmarks/locomo/
	go build -o /tmp/convomem-bench ./benchmarks/convomem/

# ---------------------------------------------------------------------------
# llama-server: local embedding endpoint
# ---------------------------------------------------------------------------

.PHONY: llama-embed
llama-embed: ## start llama-server with the embedding model on $(PORT_EMBED)
	@if [ -f "$(EMBED_PID)" ] && kill -0 $$(cat $(EMBED_PID)) 2>/dev/null; then \
		echo "llama-server (embed) already running pid=$$(cat $(EMBED_PID)) port=$(PORT_EMBED)"; \
		exit 0; \
	fi
	@if [ ! -f "$(MODELS_DIR)/$(EMBED_MODEL)" ]; then \
		echo "ERROR: model not found at $(MODELS_DIR)/$(EMBED_MODEL)"; \
		echo "Download it first (see README)."; \
		exit 1; \
	fi
	@mkdir -p $(LOG_DIR)
	@nohup llama-server \
		-m $(MODELS_DIR)/$(EMBED_MODEL) \
		--embeddings --pooling mean \
		--host $(HOST) --port $(PORT_EMBED) \
		> $(EMBED_LOG) 2>&1 & \
		echo $$! > $(EMBED_PID)
	@sleep 1.5
	@if kill -0 $$(cat $(EMBED_PID)) 2>/dev/null; then \
		echo "llama-server (embed) started pid=$$(cat $(EMBED_PID)) port=$(PORT_EMBED) model=$(EMBED_MODEL)"; \
		echo "  log:  $(EMBED_LOG)"; \
		echo "  test: curl -s -XPOST http://$(HOST):$(PORT_EMBED)/v1/embeddings -H 'Content-Type: application/json' -d '{\"input\":\"hi\"}' | head -c 200"; \
	else \
		echo "llama-server (embed) failed to start; tail of log:"; \
		tail -20 $(EMBED_LOG); \
		rm -f $(EMBED_PID); \
		exit 1; \
	fi

.PHONY: llama-rerank
llama-rerank: ## start llama-server with the reranker model on $(PORT_RERANK)
	@if [ -f "$(RERANK_PID)" ] && kill -0 $$(cat $(RERANK_PID)) 2>/dev/null; then \
		echo "llama-server (rerank) already running pid=$$(cat $(RERANK_PID)) port=$(PORT_RERANK)"; \
		exit 0; \
	fi
	@if [ ! -f "$(MODELS_DIR)/$(RERANK_MODEL)" ]; then \
		echo "ERROR: rerank model not found at $(MODELS_DIR)/$(RERANK_MODEL)"; \
		exit 1; \
	fi
	@mkdir -p $(LOG_DIR)
	@nohup llama-server \
		-m $(MODELS_DIR)/$(RERANK_MODEL) \
		--reranking \
		--host $(HOST) --port $(PORT_RERANK) \
		> $(RERANK_LOG) 2>&1 & \
		echo $$! > $(RERANK_PID)
	@sleep 1.5
	@if kill -0 $$(cat $(RERANK_PID)) 2>/dev/null; then \
		echo "llama-server (rerank) started pid=$$(cat $(RERANK_PID)) port=$(PORT_RERANK) model=$(RERANK_MODEL)"; \
		echo "  log:  $(RERANK_LOG)"; \
	else \
		echo "llama-server (rerank) failed to start; tail of log:"; \
		tail -20 $(RERANK_LOG); \
		rm -f $(RERANK_PID); \
		exit 1; \
	fi

.PHONY: llama-stop
llama-stop: ## stop both llama-server instances if running
	@for pidf in $(EMBED_PID) $(RERANK_PID); do \
		if [ -f "$$pidf" ]; then \
			pid=$$(cat $$pidf); \
			if kill -0 $$pid 2>/dev/null; then \
				kill $$pid && echo "stopped pid=$$pid ($$pidf)"; \
			fi; \
			rm -f $$pidf; \
		fi; \
	done

.PHONY: llama-status
llama-status: ## show whether llama-server processes are running
	@for label in embed rerank; do \
		pidf=$(LOG_DIR)/llama-server-$$label.pid; \
		if [ -f "$$pidf" ] && kill -0 $$(cat $$pidf) 2>/dev/null; then \
			echo "  $$label: RUNNING pid=$$(cat $$pidf)"; \
		else \
			echo "  $$label: stopped"; \
		fi; \
	done

.PHONY: llama-probe
llama-probe: ## probe the embedding endpoint (sanity check)
	@curl -s -X POST http://$(HOST):$(PORT_EMBED)/v1/embeddings \
		-H "Content-Type: application/json" \
		-d '{"input":["hello world"]}' \
		| python3 -c "import json,sys; d=json.load(sys.stdin); print('OK dim=', len(d['data'][0]['embedding']))" \
		2>/dev/null || echo "probe failed (is llama-server running on $(PORT_EMBED)?)"

# ---------------------------------------------------------------------------
# Benchmark runs against local llama-server
# ---------------------------------------------------------------------------

# Convenience env block for any LongMemEval run against the local server.
LOCAL_ENV = MEM_EMBEDDINGS_URL=http://$(HOST):$(PORT_EMBED)/v1/embeddings \
            MEM_EMBEDDINGS_MODEL=$(EMBED_MODEL) \
            MEM_EMBEDDINGS_API_KEY=

.PHONY: bench-lme-local
bench-lme-local: bench-build ## LongMemEval L# Cache max-merge against local llama-server
	$(LOCAL_ENV) LME_MODE=vector LME_LCACHE=1 LME_LCACHE_MERGE=max \
		/tmp/longmemeval-bench /tmp/longmemeval-data/longmemeval_oracle.json

.PHONY: bench-lme-cloud
bench-lme-cloud: bench-build ## LongMemEval L# max against the configured cloud provider
	@if [ -z "$$MEM_EMBEDDINGS_URL" ]; then echo "set MEM_EMBEDDINGS_URL/MODEL/API_KEY first"; exit 1; fi
	LME_MODE=vector LME_LCACHE=1 LME_LCACHE_MERGE=max \
		/tmp/longmemeval-bench /tmp/longmemeval-data/longmemeval_oracle.json
