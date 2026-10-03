package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.HttpFailure
import io.stepes.couchverse.core.HttpFailureKind
import io.stepes.couchverse.core.HttpRequest
import io.stepes.couchverse.core.HttpResponse
import io.stepes.couchverse.core.UploadRequest
import kotlinx.coroutines.suspendCancellableCoroutine
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Headers
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.io.InterruptedIOException
import java.net.ConnectException
import java.net.NoRouteToHostException
import java.net.UnknownHostException
import javax.net.ssl.SSLException
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** A file the user picked, ready to go into a multipart form. */
class UploadFile(val name: String, val body: RequestBody)

/** Turns the handle a shell gave the core (a content URI on Android) back into its file. */
fun interface UploadFiles {
    fun open(handle: String): UploadFile?
}

/** `http` and `upload` effects over OkHttp, with the system's TLS trust and proxies. */
class OkHttpExecutor(
    private val client: OkHttpClient,
    private val files: UploadFiles,
) : HttpExecutor, UploadExecutor {
    override suspend fun execute(request: HttpRequest): EffectOutput {
        val contentType = request.headers
            .firstOrNull { it.name.equals("Content-Type", ignoreCase = true) }
            ?.value?.toMediaTypeOrNull()
        // OkHttp refuses a POST, PUT or PATCH without a body
        val body = request.body?.toRequestBody(contentType)
            ?: EMPTY.takeIf { request.method in BODY_METHODS }
        return call(build(request, body))
    }

    override suspend fun upload(request: UploadRequest): EffectOutput {
        val file = files.open(request.file)
            ?: return failure(HttpFailureKind.Other, "the picked file can no longer be read")
        val form = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart(request.field, file.name, file.body)
            .build()
        return call(build(request.request, form))
    }

    private fun build(request: HttpRequest, body: RequestBody?): Request {
        val headers = Headers.Builder()
        request.headers.forEach { headers.add(it.name, it.value) }
        return Request.Builder()
            .url(request.url)
            .headers(headers.build())
            .method(request.method, body)
            .build()
    }

    private suspend fun call(request: Request): EffectOutput =
        try {
            client.newCall(request).await().use { response ->
                EffectOutput.Http(HttpResponse(response.code.toUShort(), response.body.string()))
            }
        } catch (e: IOException) {
            failure(kindOf(e), e.message ?: e.javaClass.simpleName)
        } catch (e: IllegalArgumentException) {
            // a URL OkHttp cannot parse never reaches the network
            failure(HttpFailureKind.Other, e.message ?: "invalid request")
        }

    private fun failure(kind: HttpFailureKind, message: String) =
        EffectOutput.HttpFailed(HttpFailure(kind, message))

    private companion object {
        val EMPTY = ByteArray(0).toRequestBody(null)
        val BODY_METHODS = setOf("POST", "PUT", "PATCH")

        fun kindOf(e: IOException): HttpFailureKind = when (e) {
            is UnknownHostException, is ConnectException, is NoRouteToHostException ->
                HttpFailureKind.Offline
            is SSLException -> HttpFailureKind.Tls
            // OkHttp's call and socket timeouts
            is InterruptedIOException -> HttpFailureKind.Timeout
            else -> HttpFailureKind.Other
        }
    }
}

private suspend fun Call.await(): Response = suspendCancellableCoroutine { continuation ->
    continuation.invokeOnCancellation { cancel() }
    enqueue(
        object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                continuation.resumeWithException(e)
            }

            override fun onResponse(call: Call, response: Response) {
                continuation.resume(response) { _, value, _ -> value.close() }
            }
        },
    )
}
