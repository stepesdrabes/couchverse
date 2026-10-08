package io.stepes.couchverse.design.theme

import android.content.res.Configuration
import android.os.LocaleList
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalResources
import java.util.Locale

/**
 * Shows [content] in the account's display language (D21) rather than the system's: string
 * resources, plurals and formatting all read the locale from these locals. The rest of the
 * configuration (font scale, density, UI mode) stays the system's.
 */
@Composable
fun ProvideDisplayLanguage(language: String, content: @Composable () -> Unit) {
    val context = LocalContext.current
    val configuration = LocalConfiguration.current
    val localized = remember(context, configuration, language) {
        val locale = Locale.forLanguageTag(language)
        val override = Configuration(configuration).apply { setLocales(LocaleList(locale)) }
        context.createConfigurationContext(override) to override
    }
    val (localizedContext, localizedConfiguration) = localized
    CompositionLocalProvider(
        LocalContext provides LocalizedContext(context, localizedContext),
        LocalConfiguration provides localizedConfiguration,
        LocalResources provides localizedContext.resources,
        content = content,
    )
}

/**
 * This context with its resources in the display [language], for what the app words outside its
 * screens (notifications), which would otherwise follow the system's language.
 */
fun android.content.Context.inDisplayLanguage(language: String): android.content.Context =
    createConfigurationContext(Configuration(resources.configuration).apply { setLocale(Locale.forLanguageTag(language)) })

/**
 * The activity with localized resources: navigation, permission launchers and the like still find
 * the activity behind it, while `getString` answers in the display language.
 */
private class LocalizedContext(
    base: android.content.Context,
    private val localized: android.content.Context,
) : android.content.ContextWrapper(base) {
    override fun getResources(): android.content.res.Resources = localized.resources
}
