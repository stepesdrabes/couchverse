package io.stepes.couchverse.testing

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import com.github.takahirom.roborazzi.RoborazziOptions
import com.github.takahirom.roborazzi.captureRoboImage
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.AccentColors
import io.stepes.couchverse.design.theme.CouchverseTheme
import io.stepes.couchverse.design.theme.ProvideDisplayLanguage
import org.robolectric.RuntimeEnvironment

/** The screens a screenshot is taken on: a Pixel 7 and a 1080p Google TV. */
enum class Device(val qualifiers: String, val tv: Boolean) {
    Phone(RobolectricDeviceQualifiers.Pixel7, tv = false),
    Tv(RobolectricDeviceQualifiers.Television1080p, tv = true),
}

/** Both display languages the clients ship. */
val Languages = listOf("en", "cs")

private val Options = RoborazziOptions(
    // tiny antialiasing differences between machines are not regressions
    compareOptions = RoborazziOptions.CompareOptions(changeThreshold = 0.005f),
    recordOptions = RoborazziOptions.RecordOptions(resizeScale = 0.5),
)

/**
 * Renders [content] the way the app would on [device] in [language] (theme, display language,
 * fake artwork, no motion) and compares it with `src/test/screenshots/<name>_<device>_<language>.png`
 * (`./gradlew recordRoborazziDebug` records them, `verifyRoborazziDebug` checks them).
 */
fun screenshot(
    name: String,
    device: Device,
    language: String = "en",
    accent: AccentColors = AccentColors.Default,
    content: @Composable () -> Unit,
) {
    FakeArtwork.install(ApplicationProvider.getApplicationContext())
    RuntimeEnvironment.setQualifiers(device.qualifiers)
    val file = "src/test/screenshots/${name}_${device.name.lowercase()}_$language.png"
    captureRoboImage(file, Options) {
        Fixture(device, language, accent, content)
    }
}

/** The app's surroundings for a screen under test. */
@Composable
fun Fixture(
    device: Device,
    language: String = "en",
    accent: AccentColors = AccentColors.Default,
    content: @Composable () -> Unit,
) {
    CouchverseTheme(accent = accent, tv = device.tv, reducedMotion = true) {
        ProvideDisplayLanguage(language) {
            Box(Modifier.fillMaxSize().background(Tokens.Palette.bg)) { content() }
        }
    }
}
