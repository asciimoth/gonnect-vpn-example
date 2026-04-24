package io.github.asciimoth.gonnectvpnexample

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.text.method.ScrollingMovementMethod
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.isVisible
import androidx.core.widget.doAfterTextChanged
import androidx.activity.result.contract.ActivityResultContracts
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import io.github.asciimoth.gonnectvpnexample.databinding.ActivityMainBinding
import kotlinx.coroutines.launch

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var viewModel: MainViewModel
    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startNativeVpn()
        } else {
            viewModel.markNativeVpnPermissionDenied()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        viewModel = ViewModelProvider(this)[MainViewModel::class.java]

        binding.resultText.movementMethod = ScrollingMovementMethod()
        binding.logText.movementMethod = ScrollingMovementMethod()
        binding.nativeVpnLogText.movementMethod = ScrollingMovementMethod()
        bindInputs()

        binding.connectButton.setOnClickListener {
            viewModel.connect()
        }

        binding.disconnectButton.setOnClickListener {
            viewModel.disconnect()
        }

        binding.requestButton.setOnClickListener {
            viewModel.request()
        }

        binding.pingButton.setOnClickListener {
            viewModel.ping()
        }

        binding.startNativeVpnButton.setOnClickListener {
            val prepareIntent = VpnService.prepare(this)
            if (prepareIntent != null) {
                vpnPermissionLauncher.launch(prepareIntent)
            } else {
                startNativeVpn()
            }
        }

        binding.stopNativeVpnButton.setOnClickListener {
            AndroidVpnRuntime.stop(applicationContext)
        }

        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.uiState.collect { render(it) }
            }
        }
    }

    private fun bindInputs() {
        binding.connectUrlInput.doAfterTextChanged { viewModel.updateConnectUrl(it?.toString().orEmpty()) }
        binding.tunAddrInput.doAfterTextChanged { viewModel.updateTunAddr(it?.toString().orEmpty()) }
        binding.tunSubnetInput.doAfterTextChanged { viewModel.updateTunSubnet(it?.toString().orEmpty()) }
        binding.tunNameInput.doAfterTextChanged { viewModel.updateTunName(it?.toString().orEmpty()) }
        binding.httpMethodInput.doAfterTextChanged { viewModel.updateHttpMethod(it?.toString().orEmpty()) }
        binding.httpTargetInput.doAfterTextChanged { viewModel.updateHttpTarget(it?.toString().orEmpty()) }
        binding.httpHeadersInput.doAfterTextChanged { viewModel.updateHttpHeaders(it?.toString().orEmpty()) }
        binding.httpBodyInput.doAfterTextChanged { viewModel.updateHttpBody(it?.toString().orEmpty()) }
        binding.pingTargetInput.doAfterTextChanged { viewModel.updatePingTarget(it?.toString().orEmpty()) }
    }

    private fun render(state: MainUiState) {
        applyTextIfChanged(binding.connectUrlInput, state.connectUrl)
        applyTextIfChanged(binding.tunAddrInput, state.tunAddr)
        applyTextIfChanged(binding.tunSubnetInput, state.tunSubnet)
        applyTextIfChanged(binding.tunNameInput, state.tunName)
        applyTextIfChanged(binding.httpMethodInput, state.httpMethod)
        applyTextIfChanged(binding.httpTargetInput, state.httpTarget)
        applyTextIfChanged(binding.httpHeadersInput, state.httpHeaders)
        applyTextIfChanged(binding.httpBodyInput, state.httpBody)
        applyTextIfChanged(binding.pingTargetInput, state.pingTarget)

        binding.statusText.text = state.status
        binding.logText.text = state.logs
        binding.resultText.text = state.result
        binding.errorText.text = state.error
        binding.errorText.isVisible = state.error.isNotBlank()
        binding.progressBar.isVisible = state.busy
        binding.nativeVpnStatusText.text = state.nativeVpnStatus
        binding.nativeVpnLogText.text = state.nativeVpnLogs
        binding.nativeVpnErrorText.text = state.nativeVpnError
        binding.nativeVpnErrorText.isVisible = state.nativeVpnError.isNotBlank()

        val connected = state.status == "connected"
        binding.connectButton.isEnabled = !state.busy && !connected
        binding.disconnectButton.isEnabled = !state.busy && connected
        binding.requestButton.isEnabled = !state.busy && connected
        binding.pingButton.isEnabled = !state.busy && connected
        val nativeConnected = state.nativeVpnStatus == "connected"
        binding.startNativeVpnButton.isEnabled = !state.nativeVpnBusy && !nativeConnected
        binding.stopNativeVpnButton.isEnabled = !state.nativeVpnBusy && nativeConnected
    }

    private fun applyTextIfChanged(view: android.widget.EditText, value: String) {
        if (view.text.toString() == value) {
            return
        }
        view.setText(value)
        view.setSelection(value.length)
    }

    private fun startNativeVpn() {
        val state = viewModel.uiState.value
        AndroidVpnRuntime.start(
            applicationContext,
            state.connectUrl,
            state.tunAddr,
            state.tunSubnet,
            state.tunName,
        )
    }
}
