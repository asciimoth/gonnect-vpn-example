package io.github.asciimoth.gonnectvpnexample

import android.content.Context
import android.content.Intent
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

data class AndroidVpnState(
    val status: String = "idle",
    val logs: String = "",
    val error: String = "",
    val busy: Boolean = false,
)

object AndroidVpnRuntime {
    private val mutableState = MutableStateFlow(AndroidVpnState())
    val state: StateFlow<AndroidVpnState> = mutableState.asStateFlow()

    fun start(context: Context, connectUrl: String, tunAddr: String, tunSubnet: String, tunName: String) {
        val intent = Intent(context, GonnectVpnService::class.java)
            .setAction(GonnectVpnService.ACTION_START)
            .putExtra(GonnectVpnService.EXTRA_CONNECT_URL, connectUrl)
            .putExtra(GonnectVpnService.EXTRA_TUN_ADDR, tunAddr)
            .putExtra(GonnectVpnService.EXTRA_TUN_SUBNET, tunSubnet)
            .putExtra(GonnectVpnService.EXTRA_TUN_NAME, tunName)
        context.startService(intent)
    }

    fun stop(context: Context) {
        val intent = Intent(context, GonnectVpnService::class.java)
            .setAction(GonnectVpnService.ACTION_STOP)
        context.startService(intent)
    }

    fun setBusy(status: String) {
        mutableState.update {
            it.copy(status = status, busy = true, error = "")
        }
    }

    fun setConnected(status: String, logs: String) {
        mutableState.update {
            it.copy(status = status, logs = logs, error = "", busy = false)
        }
    }

    fun setIdle(status: String = "idle", logs: String = "") {
        mutableState.update {
            it.copy(status = status, logs = logs, error = "", busy = false)
        }
    }

    fun setError(status: String, logs: String, error: String) {
        mutableState.update {
            it.copy(status = status, logs = logs, error = error, busy = false)
        }
    }
}
