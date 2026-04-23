let wasmModule = null;
let isReady = false;
let isConnected = false;

function defaultWsUrl() {
    const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${scheme}//${window.location.host}/ws-vpn`;
}

function setStatus(kind, text) {
    const statusElement = document.getElementById('status');
    statusElement.className = `status ${kind}`;
    statusElement.textContent = text;
}

function syncButtons() {
    document.getElementById('connectButton').disabled = !isReady || isConnected;
    document.getElementById('requestButton').disabled = !isReady || !isConnected;
    document.getElementById('disconnectButton').disabled = !isReady || !isConnected;
}

async function loadWasm() {
    try {
        setStatus('loading', 'Loading WASM module...');

        const go = new Go();
        const response = await fetch('app.wasm');
        const buffer = await response.arrayBuffer();

        const result = await WebAssembly.instantiate(buffer, go.importObject);
        wasmModule = result.instance;

        go.run(wasmModule);

        isReady = true;
        document.getElementById('wsUrl').value = defaultWsUrl();
        setStatus('ready', 'Ready. Connect the browser VTun client, then run requests through the VPN.');
        syncButtons();
    } catch (error) {
        console.error('Failed to load WASM:', error);
        setStatus('error', `Failed to load WASM module: ${error.message}`);
        syncButtons();
    }
}

async function connectVPN() {
    if (!isReady) {
        return;
    }

    const wsUrl = document.getElementById('wsUrl').value.trim();
    const tunAddr = document.getElementById('tunAddr').value.trim();

    if (!wsUrl) {
        alert('Please enter a VPN WebSocket URL');
        return;
    }

    setStatus('loading', 'Connecting to VPN...');
    syncButtons();

    try {
        if (typeof vpnDemoConnect !== 'function') {
            throw new Error('vpnDemoConnect function not available');
        }

        const message = await vpnDemoConnect(wsUrl, tunAddr);
        isConnected = true;
        setStatus('success', message);
        syncButtons();
    } catch (error) {
        console.error('Connect failed:', error);
        setStatus('error', `Connect failed: ${error.message || error}`);
        syncButtons();
    }
}

async function disconnectVPN() {
    try {
        if (typeof vpnDemoDisconnect === 'function') {
            await vpnDemoDisconnect();
        }
    } finally {
        isConnected = false;
        setStatus('ready', 'Disconnected. Reconnect to run more requests.');
        syncButtons();
    }
}

async function handleRequest() {
    if (!isReady || !isConnected) {
        return;
    }

    const method = document.getElementById('method').value;
    const targetUrl = document.getElementById('targetUrl').value.trim();
    const headers = document.getElementById('headers').value;
    const body = document.getElementById('body').value;
    const resultElement = document.getElementById('result');

    if (!targetUrl) {
        alert('Please enter a target URL');
        return;
    }

    setStatus('loading', `Running ${method} ${targetUrl} through VPN...`);
    resultElement.textContent = '';
    syncButtons();

    try {
        if (typeof vpnDemoRequest !== 'function') {
            throw new Error('vpnDemoRequest function not available');
        }

        const responseText = await vpnDemoRequest(method, targetUrl, headers, body);
        resultElement.textContent = responseText;
        setStatus('success', `Request completed: ${method} ${targetUrl}`);
    } catch (error) {
        console.error('Request failed:', error);
        resultElement.textContent = `Error: ${error.message || error}`;
        setStatus('error', `Request failed: ${error.message || error}`);
    }
}

window.addEventListener('load', () => {
    syncButtons();
    loadWasm();
});
