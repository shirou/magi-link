.PHONY: build clean run

WEB_DIR := web
WASM    := $(WEB_DIR)/main.wasm
WASM_EXEC := $(WEB_DIR)/wasm_exec.js
GOROOT := $(shell go env GOROOT)

build: $(WASM) $(WASM_EXEC)

$(WASM): $(shell find cmd internal -name '*.go') go.mod go.sum
	mkdir -p $(WEB_DIR)
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o $(WASM) ./cmd/game

$(WASM_EXEC):
	mkdir -p $(WEB_DIR)
	@if [ -f "$(GOROOT)/lib/wasm/wasm_exec.js" ]; then \
		cp "$(GOROOT)/lib/wasm/wasm_exec.js" $(WASM_EXEC); \
	elif [ -f "$(GOROOT)/misc/wasm/wasm_exec.js" ]; then \
		cp "$(GOROOT)/misc/wasm/wasm_exec.js" $(WASM_EXEC); \
	else \
		echo "wasm_exec.js not found in $(GOROOT)"; exit 1; \
	fi

run:
	go run ./cmd/game

clean:
	rm -f $(WASM) $(WASM_EXEC)
