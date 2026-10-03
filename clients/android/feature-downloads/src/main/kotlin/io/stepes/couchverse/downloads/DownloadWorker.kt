package io.stepes.couchverse.downloads

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.pm.ServiceInfo
import androidx.core.app.NotificationCompat
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ForegroundInfo
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import io.stepes.couchverse.design.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import java.io.File
import java.io.IOException
import java.util.concurrent.TimeUnit

/** Where finished downloads live: app-private, outside backups. */
fun downloadsDirectory(context: Context): File = File(context.filesDir, "downloads").apply { mkdirs() }

/** The partly fetched file for a download named [name]. */
internal fun partOf(directory: File, name: String) = File(directory, "$name.part")

/**
 * One download, run by WorkManager so it carries on with the app in the background or gone: a
 * foreground transfer with a progress notification, resumed from where it stopped on a retry.
 */
class DownloadWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val url = inputData.getString(URL) ?: return Result.failure()
        val name = inputData.getString(NAME) ?: return Result.failure()
        val directory = downloadsDirectory(applicationContext)
        val target = File(directory, name)
        if (target.exists()) return Result.success(workDataOf(BYTES to target.length()))
        runCatching { setForeground(foreground(0f)) }
        val part = partOf(directory, name)
        var reported = 0L
        return try {
            val bytes = withContext(Dispatchers.IO) {
                fetch(client, url, part) { received, total ->
                    val now = System.currentTimeMillis()
                    if (now - reported >= PROGRESS_MS) {
                        reported = now
                        setProgressAsync(workDataOf(RECEIVED to received, TOTAL to (total ?: -1L)))
                        if (total != null) runCatching { setForegroundAsync(foreground(received.toFloat() / total)) }
                    }
                }
            }
            if (!part.renameTo(target)) throw IOException("could not keep $name")
            Result.success(workDataOf(BYTES to bytes))
        } catch (e: Refused) {
            part.delete()
            Result.failure(workDataOf(MESSAGE to e.message))
        } catch (e: IOException) {
            when {
                isNoSpace(e) -> Result.failure(workDataOf(MESSAGE to e.message, NO_SPACE to true))
                runAttemptCount < MAX_ATTEMPTS -> Result.retry()
                else -> Result.failure(workDataOf(MESSAGE to e.message))
            }
        }
    }

    private fun foreground(fraction: Float): ForegroundInfo {
        val manager = applicationContext.getSystemService(NotificationManager::class.java)
        val title = applicationContext.getString(R.string.downloads_title)
        manager.createNotificationChannel(NotificationChannel(CHANNEL, title, NotificationManager.IMPORTANCE_LOW))
        val notification = NotificationCompat.Builder(applicationContext, CHANNEL)
            .setSmallIcon(io.stepes.couchverse.downloads.R.drawable.ic_download)
            .setContentTitle(title)
            .setProgress(100, (fraction * 100).toInt(), fraction <= 0f)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .build()
        return ForegroundInfo(id.hashCode(), notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
    }

    companion object {
        const val URL = "url"
        const val NAME = "name"
        const val RECEIVED = "received"
        const val TOTAL = "total"
        const val BYTES = "bytes"
        const val MESSAGE = "message"
        const val NO_SPACE = "noSpace"
        private const val CHANNEL = "downloads"
        private const val PROGRESS_MS = 1000L
        private const val MAX_ATTEMPTS = 5

        /** Shared by every transfer; the app's network security settings apply to it as well. */
        private val client by lazy { OkHttpClient.Builder().readTimeout(60, TimeUnit.SECONDS).build() }

        fun request(url: String, name: String) = OneTimeWorkRequestBuilder<DownloadWorker>()
            .setInputData(workDataOf(URL to url, NAME to name))
            .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
            .build()
    }
}
