package io.github.asciimoth.gonnectvpnexample

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mobilelib.Mobilelib

private const val DEFAULT_CONNECT_URL = "ws://10.0.2.2:8080/ws-vpn"
private const val DEFAULT_TUN_ADDR = "10.200.1.5"
private const val DEFAULT_TUN_SUBNET = "10.200.1.0/24"
private const val DEFAULT_TUN_NAME = "android-vtun"
private const val DEFAULT_HTTP_METHOD = "GET"
private const val DEFAULT_HTTP_TARGET = "http://10.200.1.3/"
private const val DEFAULT_PING_TARGET = "10.200.1.3"

data class MainUiState(
    val connectUrl: String = DEFAULT_CONNECT_URL,
    val tunAddr: String = DEFAULT_TUN_ADDR,
    val tunSubnet: String = DEFAULT_TUN_SUBNET,
    val tunName: String = DEFAULT_TUN_NAME,
    val httpMethod: String = DEFAULT_HTTP_METHOD,
    val httpTarget: String = DEFAULT_HTTP_TARGET,
    val httpHeaders: String = "",
    val httpBody: String = "",
    val pingTarget: String = DEFAULT_PING_TARGET,
    val nativeVpnStatus: String = "idle",
    val nativeVpnLogs: String = "",
    val nativeVpnError: String = "",
    val nativeVpnBusy: Boolean = false,
    val status: String = "idle",
    val logs: String = "",
    val result: String = "",
    val error: String = "",
    val busy: Boolean = false,
)

class MainViewModel : ViewModel() {
    private val client = Mobilelib.newClient()
    private val _uiState = MutableStateFlow(
        MainUiState(
            status = client.status(),
            logs = client.logs(),
        ),
    )

    val uiState: StateFlow<MainUiState> = _uiState.asStateFlow()

    init {
        viewModelScope.launch {
            AndroidVpnRuntime.state.collectLatest { nativeState ->
                _uiState.update {
                    it.copy(
                        nativeVpnStatus = nativeState.status,
                        nativeVpnLogs = nativeState.logs,
                        nativeVpnError = nativeState.error,
                        nativeVpnBusy = nativeState.busy,
                    )
                }
            }
        }
    }

    fun updateConnectUrl(value: String) = _uiState.update { it.copy(connectUrl = value) }
    fun updateTunAddr(value: String) = _uiState.update { it.copy(tunAddr = value) }
    fun updateTunSubnet(value: String) = _uiState.update { it.copy(tunSubnet = value) }
    fun updateTunName(value: String) = _uiState.update { it.copy(tunName = value) }
    fun updateHttpMethod(value: String) = _uiState.update { it.copy(httpMethod = value) }
    fun updateHttpTarget(value: String) = _uiState.update { it.copy(httpTarget = value) }
    fun updateHttpHeaders(value: String) = _uiState.update { it.copy(httpHeaders = value) }
    fun updateHttpBody(value: String) = _uiState.update { it.copy(httpBody = value) }
    fun updatePingTarget(value: String) = _uiState.update { it.copy(pingTarget = value) }

    fun connect() {
        launchAction {
            client.connect(
                uiState.value.connectUrl,
                uiState.value.tunAddr,
                uiState.value.tunName,
            )
            "Connected"
        }
    }

    fun disconnect() {
        launchAction {
            client.disconnect()
            "Disconnected"
        }
    }

    fun request() {
        launchAction {
            client.request(
                uiState.value.httpMethod,
                uiState.value.httpTarget,
                uiState.value.httpHeaders,
                uiState.value.httpBody,
            )
        }
    }

    fun ping() {
        launchAction {
            client.ping(uiState.value.pingTarget)
        }
    }

    fun markNativeVpnPermissionDenied() {
        _uiState.update {
            it.copy(
                nativeVpnError = "VPN permission was not granted",
                nativeVpnBusy = false,
            )
        }
    }

    override fun onCleared() {
        runCatching { client.disconnect() }
        super.onCleared()
    }

    private fun launchAction(action: suspend () -> String) {
        if (uiState.value.busy) {
            return
        }

        _uiState.update { it.copy(busy = true, error = "") }
        viewModelScope.launch {
            try {
                val result = withContext(Dispatchers.IO) { action() }
                _uiState.update { it.copy(result = result, error = "") }
            } catch (err: Throwable) {
                _uiState.update { it.copy(error = err.message ?: err.toString()) }
            } finally {
                refreshClientState()
            }
        }
    }

    private fun refreshClientState() {
        _uiState.update {
            it.copy(
                status = client.status(),
                logs = client.logs(),
                busy = false,
            )
        }
    }
}
