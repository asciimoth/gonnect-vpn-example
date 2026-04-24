package io.github.asciimoth.gonnectvpnexample

import android.app.Service
import android.content.Intent
import android.net.VpnService
import android.os.IBinder
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import mobilelib.AndroidVPNClient
import mobilelib.Mobilelib
import mobilelib.SocketProtector
import java.net.Inet4Address
import java.net.InetAddress

class GonnectVpnService : VpnService() {
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var client: AndroidVPNClient? = null
    private var statusPollerRunning = false

    override fun onBind(intent: Intent?): IBinder? = super.onBind(intent)

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val connectUrl = intent.getStringExtra(EXTRA_CONNECT_URL).orEmpty()
                val tunAddr = intent.getStringExtra(EXTRA_TUN_ADDR).orEmpty()
                val tunSubnet = intent.getStringExtra(EXTRA_TUN_SUBNET).orEmpty()
                val tunName = intent.getStringExtra(EXTRA_TUN_NAME).orEmpty()
                startVpn(connectUrl, tunAddr, tunSubnet, tunName)
            }
            ACTION_STOP -> {
                stopVpn()
            }
        }
        return Service.START_STICKY
    }

    override fun onDestroy() {
        stopClient()
        serviceScope.cancel()
        super.onDestroy()
    }

    private fun startVpn(connectUrl: String, tunAddr: String, tunSubnet: String, tunName: String) {
        AndroidVpnRuntime.setBusy("starting android vpn")
        serviceScope.launch(Dispatchers.IO) {
            try {
                stopClient()

                val address = parseIpv4Address(tunAddr)
                val route = parseSubnet(tunSubnet)
                val tunFd = buildTunFd(address, route, tunName)
                val nextClient = Mobilelib.newAndroidVPNClient()
                nextClient.connect(
                    connectUrl,
                    tunFd.toInt(),
                    tunName,
                    ServiceSocketProtector(this@GonnectVpnService),
                )

                client = nextClient
                publishState()
                startStatusPoller()
            } catch (err: Throwable) {
                val currentClient = client
                AndroidVpnRuntime.setError(
                    status = currentClient?.status() ?: "android vpn failed",
                    logs = currentClient?.logs().orEmpty(),
                    error = err.message ?: err.toString(),
                )
            }
        }
    }

    private fun stopVpn() {
        AndroidVpnRuntime.setBusy("stopping android vpn")
        serviceScope.launch(Dispatchers.IO) {
            stopClient()
            AndroidVpnRuntime.setIdle()
            stopSelf()
        }
    }

    private fun stopClient() {
        val current = client
        client = null
        current?.disconnect()
    }

    private fun buildTunFd(address: TunAddress, route: TunRoute, tunName: String): Long {
        val builder = Builder()
            .setSession(if (tunName.isBlank()) "gonnect-android-vpn" else tunName)
            .setMtu(DEFAULT_MTU)
            .addAddress(address.hostAddress, address.prefixLength)
            .addRoute(route.networkAddress, route.prefixLength)

        val descriptor = builder.establish()
            ?: throw IllegalStateException("VpnService establish returned null")
        return descriptor.detachFd().toLong()
    }

    private fun startStatusPoller() {
        if (statusPollerRunning) {
            return
        }
        statusPollerRunning = true
        serviceScope.launch {
            try {
                while (isActive) {
                    publishState()
                    delay(1000)
                }
            } finally {
                statusPollerRunning = false
            }
        }
    }

    private fun publishState() {
        val current = client
        if (current == null) {
            AndroidVpnRuntime.setIdle()
            return
        }
        AndroidVpnRuntime.setConnected(
            status = current.status(),
            logs = current.logs(),
        )
    }

    private data class TunAddress(
        val hostAddress: String,
        val prefixLength: Int,
    )

    private data class TunRoute(
        val networkAddress: String,
        val prefixLength: Int,
    )

    private class ServiceSocketProtector(
        private val service: VpnService,
    ) : SocketProtector {
        override fun protect(fd: Int): Boolean = service.protect(fd)
    }

    companion object {
        const val ACTION_START = "io.github.asciimoth.gonnectvpnexample.action.START_VPN"
        const val ACTION_STOP = "io.github.asciimoth.gonnectvpnexample.action.STOP_VPN"

        const val EXTRA_CONNECT_URL = "connect_url"
        const val EXTRA_TUN_ADDR = "tun_addr"
        const val EXTRA_TUN_SUBNET = "tun_subnet"
        const val EXTRA_TUN_NAME = "tun_name"

        private const val DEFAULT_MTU = 1500

        private fun parseIpv4Address(text: String): TunAddress {
            val trimmed = text.trim()
            if (trimmed.isEmpty()) {
                throw IllegalArgumentException("tun address is required")
            }

            val parts = trimmed.split("/", limit = 2)
            val host = parseIPv4(parts[0])
            val prefixLength = if (parts.size == 2) {
                parts[1].toInt()
            } else {
                24
            }

            return TunAddress(host.hostAddress ?: parts[0], prefixLength)
        }

        private fun parseSubnet(text: String): TunRoute {
            val trimmed = text.trim()
            if (trimmed.isEmpty()) {
                throw IllegalArgumentException("tun subnet is required")
            }

            val parts = trimmed.split("/", limit = 2)
            if (parts.size != 2) {
                throw IllegalArgumentException("tun subnet must be in CIDR form")
            }

            val network = parseIPv4(parts[0])
            return TunRoute(network.hostAddress ?: parts[0], parts[1].toInt())
        }

        private fun parseIPv4(text: String): Inet4Address {
            val addr = InetAddress.getByName(text.trim())
            return addr as? Inet4Address
                ?: throw IllegalArgumentException("only IPv4 addresses are supported for now")
        }
    }
}
