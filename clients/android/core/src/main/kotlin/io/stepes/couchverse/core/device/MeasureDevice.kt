package io.stepes.couchverse.core.device

import android.content.Context
import android.hardware.display.DisplayManager
import android.media.AudioDeviceInfo
import android.media.AudioManager
import android.media.MediaCodecList
import android.view.Display

/** Reads the decoders, the default display's HDR modes and the audio outputs' encodings. */
fun measureDevice(context: Context): DeviceCaps {
    val decoders = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos
        .filterNot { it.isEncoder }
        .flatMap { info ->
            info.supportedTypes.map { mime ->
                val caps = info.getCapabilitiesForType(mime)
                val video = caps.videoCapabilities
                Decoder(
                    mime = mime.lowercase(),
                    profiles = caps.profileLevels.mapTo(mutableSetOf()) { it.profile },
                    maxWidth = video?.supportedWidths?.upper ?: 0,
                    maxHeight = video?.supportedHeights?.upper ?: 0,
                    maxChannels = caps.audioCapabilities?.maxInputChannelCount ?: 0,
                )
            }
        }
    val display = context.getSystemService(DisplayManager::class.java)?.getDisplay(Display.DEFAULT_DISPLAY)
    @Suppress("DEPRECATION") // the replacement (Display.Mode.getSupportedHdrTypes) needs API 34
    val hdr = display?.hdrCapabilities?.supportedHdrTypes?.toSet().orEmpty()
    val audio = context.getSystemService(AudioManager::class.java)
    val passthrough = audio?.getDevices(AudioManager.GET_DEVICES_OUTPUTS).orEmpty()
        .filter { it.type == AudioDeviceInfo.TYPE_HDMI || it.type == AudioDeviceInfo.TYPE_HDMI_ARC || it.type == AudioDeviceInfo.TYPE_HDMI_EARC }
        .flatMapTo(mutableSetOf()) { it.encodings.toList() }
    return DeviceCaps(decoders, hdr, passthrough)
}
