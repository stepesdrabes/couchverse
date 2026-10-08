package io.stepes.couchverse.couch

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import io.stepes.couchverse.core.CouchRole
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.runtime.CoreRuntime
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.theme.inDisplayLanguage

/** The application, which owns the one runtime a notification's buttons talk to. */
interface CoreHost {
    val runtime: CoreRuntime
}

/**
 * The ongoing notification while this device is on a couch: how many are on it, the code to
 * share, and a button to leave (for the host, to end it for everyone). On Android 16 it asks to
 * be promoted to a Live Update.
 */
class CouchNotification(private val context: Context) {
    private val manager = NotificationManagerCompat.from(context)
    private var channel = false

    /** Shows [view]'s session, or takes the notification away when there is none. */
    fun show(view: CouchView?, language: String) {
        if (view == null || !view.active) {
            manager.cancel(ID)
            return
        }
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return
        val words = context.inDisplayLanguage(language)
        if (!channel) {
            context.getSystemService(NotificationManager::class.java)
                .createNotificationChannel(NotificationChannel(CHANNEL, words.getString(R.string.couch_open), NotificationManager.IMPORTANCE_LOW))
            channel = true
        }
        val host = view.role == CouchRole.Host
        val open = context.packageManager.getLaunchIntentForPackage(context.packageName)
            ?.let { PendingIntent.getActivity(context, 0, it, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT) }
        val leave = PendingIntent.getBroadcast(
            context,
            0,
            Intent(context, CouchActionReceiver::class.java).putExtra(EXTRA_END, host),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val text = listOfNotNull(
            couchStatusLine(view, words),
            view.code?.let { words.getString(R.string.couch_code, spacedCode(it)) }.takeIf { host },
        ).joinToString(" · ")
        val notification = NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(io.stepes.couchverse.couch.R.drawable.ic_couch)
            .setContentTitle(words.getString(R.string.couch_on_couch_count, view.members.size.toString()))
            .setContentText(text)
            .setContentIntent(open)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(NotificationCompat.CATEGORY_STATUS)
            .setRequestPromotedOngoing(true)
            .addAction(0, words.getString(if (host) R.string.couch_end_session else R.string.couch_leave), leave)
            .build()
        runCatching { manager.notify(ID, notification) }
    }

    private fun couchStatusLine(view: CouchView, words: Context): String? = when {
        view.status == io.stepes.couchverse.core.CouchStatus.Reconnecting -> words.getString(R.string.couch_reconnecting)
        view.hostAway -> words.getString(R.string.couch_host_away)
        else -> null
    }

    internal companion object {
        const val ID = 4207
        const val CHANNEL = "couch"
        const val EXTRA_END = "end"
    }
}

/** The notification's button: leaving the session, or ending it for the host. */
class CouchActionReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val host = context.applicationContext as? CoreHost ?: return
        host.runtime.send(if (intent.getBooleanExtra(CouchNotification.EXTRA_END, false)) Event.CouchEndRequested else Event.CouchLeft)
    }
}
