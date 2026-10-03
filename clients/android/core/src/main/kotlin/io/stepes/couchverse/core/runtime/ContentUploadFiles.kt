package io.stepes.couchverse.core.runtime

import android.content.ContentResolver
import android.net.Uri
import android.provider.OpenableColumns
import android.webkit.MimeTypeMap
import okhttp3.MediaType
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.RequestBody
import okio.BufferedSink
import okio.source

/**
 * Picked files are content URIs; the URI string is the handle the core carries. The form part
 * is named after the file with an extension that matches its type, because servers type images
 * by the extension.
 */
class ContentUploadFiles(private val resolver: ContentResolver) : UploadFiles {
    override fun open(handle: String): UploadFile? {
        val uri = Uri.parse(handle)
        val type = resolver.getType(uri)
        var name: String? = null
        var size = -1L
        resolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE), null, null, null)
            ?.use { cursor ->
                if (cursor.moveToFirst()) {
                    name = cursor.getString(0)
                    if (!cursor.isNull(1)) size = cursor.getLong(1)
                }
            }
        val extension = type?.let { MimeTypeMap.getSingleton().getExtensionFromMimeType(it) }
        val base = name ?: uri.lastPathSegment ?: "upload"
        val fileName = if (extension == null || base.endsWith(".$extension", ignoreCase = true)) {
            base
        } else {
            "${base.substringBeforeLast('.')}.$extension"
        }
        return UploadFile(fileName, ContentBody(resolver, uri, type?.toMediaTypeOrNull(), size))
    }

    private class ContentBody(
        private val resolver: ContentResolver,
        private val uri: Uri,
        private val type: MediaType?,
        private val size: Long,
    ) : RequestBody() {
        override fun contentType() = type

        override fun contentLength() = size

        override fun writeTo(sink: BufferedSink) {
            val stream = resolver.openInputStream(uri) ?: throw java.io.FileNotFoundException("$uri")
            stream.source().use { sink.writeAll(it) }
        }
    }
}
