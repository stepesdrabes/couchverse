package io.stepes.couchverse.navigation

import android.content.Intent
import android.net.Uri
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import io.stepes.couchverse.couch.couchCode

/** What a `couchverse://` link asks for. */
sealed interface AppLink {
    /** Sign this device in with a "Connect a device" code; the core takes the whole link. */
    data class Connect(val url: String) : AppLink

    /** Approve another device's sign-in. */
    data class Pair(val url: String) : AppLink

    data class OpenTitle(val slug: String) : AppLink

    /** Join a couch session by its code. */
    data class Couch(val code: String) : AppLink

    companion object {
        fun parse(url: String): AppLink? {
            val uri = Uri.parse(url)
            if (uri.scheme != "couchverse") return null
            return when (uri.host) {
                "connect" -> Connect(url)
                "pair" -> Pair(url)
                "title" -> uri.pathSegments.firstOrNull()?.let(::OpenTitle)
                "couch" -> couchCode(url)?.let(::Couch)
                else -> null
            }
        }
    }
}

/** The link the app was opened with, held until the screens can act on it. */
class PendingLinks {
    var link by mutableStateOf<AppLink?>(null)
        private set

    fun offer(intent: Intent?) {
        intent?.dataString?.let(AppLink::parse)?.let { link = it }
    }

    fun consume(): AppLink? = link.also { link = null }
}
