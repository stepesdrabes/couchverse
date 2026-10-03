package io.stepes.couchverse

import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import io.stepes.couchverse.navigation.PendingLinks
import io.stepes.couchverse.ui.CouchverseRoot

class MainActivity : ComponentActivity() {
    private val links = PendingLinks()

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge(SystemBarStyle.dark(Color.TRANSPARENT), SystemBarStyle.dark(Color.TRANSPARENT))
        super.onCreate(savedInstanceState)
        // a recreated activity already handled the link it was started with
        if (savedInstanceState == null) links.offer(intent)
        val app = application as CouchverseApp
        setContent { CouchverseRoot(app.runtime, app.tv, links, BuildConfig.VERSION_NAME) }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        links.offer(intent)
    }
}
