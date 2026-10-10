package io.stepes.couchverse.settings

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.design.R

/** A display language by its own name, so anyone can find theirs. */
@Composable
fun languageName(code: String): String = when (code) {
    "cs" -> stringResource(R.string.lang_czech)
    else -> stringResource(R.string.lang_english)
}

/** The device platform as people know it. */
@Composable
fun platformName(platform: String): String = stringResource(
    when (platform) {
        "ios" -> R.string.devices_platform_ios
        "ipados" -> R.string.devices_platform_ipados
        "tvos" -> R.string.devices_platform_tvos
        "android" -> R.string.devices_platform_android
        "androidtv" -> R.string.devices_platform_androidtv
        "desktop" -> R.string.devices_platform_desktop
        else -> R.string.devices_platform_web
    },
)
