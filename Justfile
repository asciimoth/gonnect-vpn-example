set shell := ["bash", "-euo", "pipefail", "-c"]
cli_bin := if os_family() == "windows" { "vpn.exe" } else { "vpn" }

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
    go build -o {{cli_bin}} ./cli

build-gui: build-web
    go build -tags novulkan -o gonnect-vpn-gui ./gui

build-android-aar:
    ./scripts/build-android-mobilelib-aar.sh

build-android-apk: build-android-aar
    mkdir -p build
    cd android && ./gradlew --no-daemon assembleDebug
    cp -f android/app/build/outputs/apk/debug/app-debug.apk build/gonnect-vpn-android.apk

install-android-apk: build-android-apk
    adb install -r build/gonnect-vpn-android.apk

uninstall-android-apk:
    adb uninstall io.github.asciimoth.gonnectvpnexample

run-gui: build-gui
    ./gonnect-vpn-gui

# Start HTTP server for demo
serve: build-cli
    @echo "Starting HTTP server on http://localhost:9090"
    @echo "Press Ctrl+C to stop"
    @echo "Open http://127.0.0.1:9090 in your browser"
    ./{{cli_bin}} --serve 127.0.0.1:9090 --tun vtun+http

# Prepare for GitHub Pages
gh-pages: build-web
    @echo "Preparing for GitHub Pages..."
    @mkdir -p ./pages
    @cp -f ./web/index.html ./web/main.js ./web/app.wasm ./web/wasm_exec.js ./pages
    @touch ./pages/.nojekyll
    @echo "GitHub Pages ready in webdemo/pages"

clean:
    chmod -R u+w build android/.gradle android/build android/app/build 2>/dev/null || true
    rm -rf build
    rm -rf android/.gradle android/build android/app/build
    rm -f {{cli_bin}} gonnect-vpn-gui web/app.wasm web/wasm_exec.js
