package io.stepes.couchverse.downloads

import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.File
import java.io.FileOutputStream
import java.io.IOException

/** The server refused the file (an expired grant, a download it no longer has). */
class Refused(val code: Int) : IOException("the server answered $code")

/**
 * Fetches [url] into [part], resuming what an earlier attempt left there when the server
 * honours a range. [progress] hears the bytes received so far and the total, when known.
 * Returns the file's size.
 */
fun fetch(client: OkHttpClient, url: String, part: File, progress: (received: Long, total: Long?) -> Unit): Long {
    val have = if (part.exists()) part.length() else 0L
    val request = Request.Builder().url(url).apply { if (have > 0) header("Range", "bytes=$have-") }.build()
    client.newCall(request).execute().use { response ->
        if (response.code == 416 && have > 0) {
            // what was kept is already the whole file
            return have
        }
        if (!response.isSuccessful) throw Refused(response.code)
        val resumed = response.code == 206
        val body = response.body
        val start = if (resumed) have else 0L
        val total = body.contentLength().takeIf { it >= 0 }?.plus(start)
        var received = start
        FileOutputStream(part, resumed).use { out ->
            val buffer = ByteArray(BUFFER)
            body.byteStream().use { input ->
                while (true) {
                    val read = input.read(buffer)
                    if (read < 0) break
                    out.write(buffer, 0, read)
                    received += read
                    progress(received, total)
                }
            }
        }
        return received
    }
}

/** Whether [error] means the device is out of space, which retrying will not fix. */
fun isNoSpace(error: Throwable): Boolean =
    generateSequence(error) { it.cause }.any { it.message?.contains("ENOSPC") == true || it.message?.contains("No space left") == true }

private const val BUFFER = 64 * 1024
