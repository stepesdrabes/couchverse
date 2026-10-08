package io.stepes.couchverse.downloads

import androidx.work.ExistingWorkPolicy
import androidx.work.WorkInfo
import androidx.work.WorkManager
import io.stepes.couchverse.core.DownloadFailure
import io.stepes.couchverse.core.DownloadFinished
import io.stepes.couchverse.core.DownloadProgress
import io.stepes.couchverse.core.DownloadStart
import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.runtime.DownloadExecutor
import io.stepes.couchverse.core.runtime.DownloadTransfer
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.takeWhile
import kotlinx.coroutines.launch
import java.io.File

/**
 * The core's downloads as WorkManager transfers into [directory]. A transfer is unique by its
 * file's name, so a start after a relaunch attaches to the one already running (or finds the
 * finished file) instead of fetching again. Its notification speaks the app's display
 * [language] as it was when the transfer was asked for.
 */
class WorkDownloads(
    private val work: WorkManager,
    private val directory: File,
    private val language: () -> String,
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
) : DownloadExecutor {
    override fun start(request: DownloadStart, events: (EffectOutput) -> Unit): DownloadTransfer {
        val name = request.name
        val unique = "download:$name"
        val job = scope.launch {
            val file = File(directory, name)
            if (request.url.isEmpty()) {
                val running = work.getWorkInfosForUniqueWorkFlow(unique).first().lastOrNull()?.state?.isFinished == false
                if (!running) {
                    events(if (file.exists()) finished(file.length()) else failed("no transfer of $name"))
                    return@launch
                }
            } else {
                work.enqueueUniqueWork(unique, ExistingWorkPolicy.KEEP, DownloadWorker.request(request.url, name, language()))
            }
            var reported = -1L
            work.getWorkInfosForUniqueWorkFlow(unique)
                .takeWhile { infos ->
                    val info = infos.lastOrNull() ?: return@takeWhile true
                    when (info.state) {
                        WorkInfo.State.RUNNING -> {
                            val received = info.progress.getLong(DownloadWorker.RECEIVED, -1)
                            val total = info.progress.getLong(DownloadWorker.TOTAL, -1).takeIf { it >= 0 }
                            if (received >= 0 && received != reported) {
                                reported = received
                                events(EffectOutput.DownloadProgress(DownloadProgress(received.toULong(), total?.toULong())))
                            }
                            true
                        }
                        WorkInfo.State.SUCCEEDED -> {
                            events(finished(info.outputData.getLong(DownloadWorker.BYTES, file.length())))
                            false
                        }
                        WorkInfo.State.FAILED, WorkInfo.State.CANCELLED -> {
                            val message = info.outputData.getString(DownloadWorker.MESSAGE) ?: info.state.name.lowercase()
                            events(failed(message, info.outputData.getBoolean(DownloadWorker.NO_SPACE, false)))
                            false
                        }
                        WorkInfo.State.ENQUEUED, WorkInfo.State.BLOCKED -> true
                    }
                }
                .collect {}
        }
        return DownloadTransfer {
            job.cancel()
            work.cancelUniqueWork(unique)
            partOf(directory, name).delete()
        }
    }

    override fun remove(name: String) {
        File(directory, name).delete()
        partOf(directory, name).delete()
    }

    private fun finished(bytes: Long) = EffectOutput.DownloadFinished(DownloadFinished(bytes.toULong()))

    private fun failed(message: String, noSpace: Boolean = false) =
        EffectOutput.DownloadFailed(DownloadFailure(message, noSpace.takeIf { it }))
}
