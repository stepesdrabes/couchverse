package io.stepes.couchverse.accounts.phone

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalInspectionMode
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.zxing.BarcodeFormat
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.ReaderException
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader
import io.stepes.couchverse.accounts.isCouchverseLink
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import kotlinx.coroutines.delay
import java.util.concurrent.Executors

/**
 * The camera, looking for a Couchverse QR code: a web "Connect a device" code (signs this phone
 * in) or a TV's pairing code (approves it). [onLink] gets the first one found; other QR codes
 * only get a note that they are not Couchverse's.
 */
@Composable
fun QrScannerScreen(hint: String, onLink: (String) -> Unit, onBack: () -> Unit) {
    val context = LocalContext.current
    var granted by remember { mutableStateOf(hasCamera(context)) }
    var asked by remember { mutableStateOf(false) }
    val request = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        granted = it
        asked = true
    }
    LaunchedEffect(Unit) { if (!granted) request.launch(Manifest.permission.CAMERA) }
    var foreign by remember { mutableStateOf<String?>(null) }
    var done by remember { mutableStateOf(false) }
    val onDecoded: (String) -> Unit = { text ->
        if (!done) {
            if (isCouchverseLink(text)) {
                done = true
                onLink(text.trim())
            } else {
                foreign = text
            }
        }
    }
    LaunchedEffect(foreign) {
        if (foreign != null) {
            delay(2_500)
            foreign = null
        }
    }

    Box(Modifier.fillMaxSize().background(Color.Black)) {
        if (granted && !LocalInspectionMode.current) {
            CameraPreview(onDecoded, Modifier.fillMaxSize())
        }
        ViewfinderOverlay(Modifier.fillMaxSize())
        IconButton(onClick = onBack, modifier = Modifier.safeDrawingPadding().padding(8.dp)) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back), tint = Color.White)
        }
        Column(
            Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .safeDrawingPadding()
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            if (!granted) {
                Text(
                    stringResource(R.string.scanner_camera_needed),
                    style = MaterialTheme.typography.bodyLarge,
                    color = Color.White,
                    textAlign = TextAlign.Center,
                )
                Button(
                    onClick = {
                        if (asked && !canAsk(context)) openAppSettings(context) else request.launch(Manifest.permission.CAMERA)
                    },
                ) { Text(stringResource(R.string.scanner_allow_camera)) }
            } else {
                Text(
                    foreign?.let { stringResource(R.string.scanner_not_couchverse) } ?: hint,
                    style = MaterialTheme.typography.bodyLarge,
                    color = if (foreign != null) Tokens.Palette.danger else Color.White,
                    textAlign = TextAlign.Center,
                    modifier = Modifier
                        .background(Color.Black.copy(alpha = 0.55f), RoundedCornerShape(12.dp))
                        .padding(horizontal = 16.dp, vertical = 10.dp)
                        .semantics { liveRegion = LiveRegionMode.Polite },
                )
            }
        }
    }
}

@Composable
private fun CameraPreview(onDecoded: (String) -> Unit, modifier: Modifier) {
    val lifecycleOwner = LocalLifecycleOwner.current
    val latest by rememberUpdatedState(onDecoded)
    val analysisThread = remember { Executors.newSingleThreadExecutor() }
    DisposableEffect(Unit) { onDispose { analysisThread.shutdown() } }
    AndroidView(
        modifier = modifier,
        factory = { context ->
            PreviewView(context).also { view ->
                val providerFuture = ProcessCameraProvider.getInstance(context)
                val main = ContextCompat.getMainExecutor(context)
                providerFuture.addListener(
                    {
                        val provider = providerFuture.get()
                        val preview = Preview.Builder().build().also { it.surfaceProvider = view.surfaceProvider }
                        val analysis = ImageAnalysis.Builder()
                            .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                            .build()
                        analysis.setAnalyzer(analysisThread, QrAnalyzer { text -> main.execute { latest(text) } })
                        provider.unbindAll()
                        provider.bindToLifecycle(lifecycleOwner, CameraSelector.DEFAULT_BACK_CAMERA, preview, analysis)
                    },
                    main,
                )
            }
        },
    )
}

/** A dimmed screen with a clear, framed square where the code should go. */
@Composable
private fun ViewfinderOverlay(modifier: Modifier) {
    Canvas(modifier.graphicsLayer { compositingStrategy = CompositingStrategy.Offscreen }) {
        val side = size.minDimension * 0.68f
        val topLeft = Offset((size.width - side) / 2, (size.height - side) / 2.4f)
        val corner = CornerRadius(28.dp.toPx())
        drawRect(Color.Black.copy(alpha = 0.55f))
        drawRoundRect(Color.Transparent, topLeft, Size(side, side), corner, blendMode = BlendMode.Clear)
        drawRoundRect(Color.White.copy(alpha = 0.9f), topLeft, Size(side, side), corner, style = Stroke(3.dp.toPx()))
    }
}

/** Finds a QR code in the luminance plane of camera frames. */
private class QrAnalyzer(private val onCode: (String) -> Unit) : ImageAnalysis.Analyzer {
    private val reader = QRCodeReader()
    private val hints = mapOf(
        DecodeHintType.POSSIBLE_FORMATS to listOf(BarcodeFormat.QR_CODE),
        DecodeHintType.TRY_HARDER to true,
    )

    override fun analyze(image: ImageProxy) {
        image.use {
            val plane = it.planes[0]
            val buffer = plane.buffer
            val bytes = ByteArray(buffer.remaining()).also { data -> buffer.get(data) }
            val source = PlanarYUVLuminanceSource(bytes, plane.rowStride, it.height, 0, 0, it.width, it.height, false)
            try {
                onCode(reader.decode(BinaryBitmap(HybridBinarizer(source)), hints).text)
            } catch (_: ReaderException) {
                // no code in this frame
            } finally {
                reader.reset()
            }
        }
    }
}

private fun hasCamera(context: Context) =
    ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED

private fun canAsk(context: Context): Boolean {
    val activity = generateSequence(context) { (it as? android.content.ContextWrapper)?.baseContext }
        .filterIsInstance<android.app.Activity>()
        .firstOrNull() ?: return true
    return activity.shouldShowRequestPermissionRationale(Manifest.permission.CAMERA)
}

private fun openAppSettings(context: Context) {
    context.startActivity(
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.fromParts("package", context.packageName, null))
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
    )
}
