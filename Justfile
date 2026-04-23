set shell := ["bash", "-euo", "pipefail", "-c"]

build-web:
    @echo "Copying wasm_exec.js..."
    @if [ -f "$(go env GOROOT)/misc/wasm/wasm_exec.js" ]; then \
        cp -f "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./web/wasm_exec.js; \
    elif [ -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ]; then \
        cp -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ./web/wasm_exec.js; \
    else \
        echo "Error: Could not find wasm_exec.js in Go installation"; \
        exit 1; \
    fi
    @echo "Building WebAssembly module..."
    GOOS=js GOARCH=wasm go build -o web/app.wasm ./web/wasm

build-cli: build-web
    go build -o vpn ./cli

# Start HTTP server for demo
serve: build-cli
    @echo "Starting HTTP server on http://localhost:9090"
    @echo "Press Ctrl+C to stop"
    @echo "Open http://127.0.0.1:9090 in your browser"
    ./vpn --serve 127.0.0.1:9090 --tun vtun+http

clean:
    rm -f vpn web/app.wasm web/wasm_exec.js
