package io.github.asciimoth.gonnectvpnexample

import android.os.Bundle
import android.text.method.ScrollingMovementMethod
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.isVisible
import androidx.core.widget.doAfterTextChanged
import io.github.asciimoth.gonnectvpnexample.databinding.ActivityMainBinding
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mobilelib.Client
import mobilelib.Mobilelib

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var client: Client
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        client = Mobilelib.newClient()
        binding.resultText.movementMethod = ScrollingMovementMethod()
        binding.logText.movementMethod = ScrollingMovementMethod()

        binding.connectUrlInput.setText("ws://10.0.2.2:8080/ws-vpn")
        binding.tunAddrInput.setText("10.200.1.5")
        binding.tunNameInput.setText("android-vtun")
        binding.httpMethodInput.setText("GET")
        binding.httpTargetInput.setText("http://10.200.1.3/")
        binding.pingTargetInput.setText("10.200.1.3")

        val clearError = {
            binding.errorText.isVisible = false
            binding.errorText.text = ""
        }
        listOf(
            binding.connectUrlInput,
            binding.tunAddrInput,
            binding.tunNameInput,
            binding.httpMethodInput,
            binding.httpTargetInput,
            binding.httpHeadersInput,
            binding.httpBodyInput,
            binding.pingTargetInput,
        ).forEach { view ->
            view.doAfterTextChanged { clearError() }
        }

        binding.connectButton.setOnClickListener {
            runAction {
                client.connect(
                    binding.connectUrlInput.text.toString(),
                    binding.tunAddrInput.text.toString(),
                    binding.tunNameInput.text.toString(),
                )
                "Connected"
            }
        }

        binding.disconnectButton.setOnClickListener {
            runAction {
                client.disconnect()
                "Disconnected"
            }
        }

        binding.requestButton.setOnClickListener {
            runAction {
                client.request(
                    binding.httpMethodInput.text.toString(),
                    binding.httpTargetInput.text.toString(),
                    binding.httpHeadersInput.text.toString(),
                    binding.httpBodyInput.text.toString(),
                )
            }
        }

        binding.pingButton.setOnClickListener {
            runAction {
                client.ping(binding.pingTargetInput.text.toString())
            }
        }

        refreshClientState()
    }

    override fun onDestroy() {
        runCatching { client.disconnect() }
        scope.cancel()
        super.onDestroy()
    }

    private fun runAction(action: suspend () -> String) {
        setBusy(true)
        scope.launch {
            try {
                val result = withContext(Dispatchers.IO) { action() }
                binding.resultText.text = result
                binding.errorText.isVisible = false
            } catch (err: Throwable) {
                binding.errorText.text = err.message ?: err.toString()
                binding.errorText.isVisible = true
                Toast.makeText(this@MainActivity, "Action failed", Toast.LENGTH_SHORT).show()
            } finally {
                refreshClientState()
                setBusy(false)
            }
        }
    }

    private fun refreshClientState() {
        binding.statusText.text = client.status()
        binding.logText.text = client.logs()
    }

    private fun setBusy(busy: Boolean) {
        binding.progressBar.isVisible = busy
        listOf(
            binding.connectButton,
            binding.disconnectButton,
            binding.requestButton,
            binding.pingButton,
        ).forEach { it.isEnabled = !busy }
    }
}
