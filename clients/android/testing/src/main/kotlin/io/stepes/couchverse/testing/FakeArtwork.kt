package io.stepes.couchverse.testing

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.LinearGradient
import android.graphics.Paint
import android.graphics.Shader
import android.graphics.Typeface
import androidx.core.graphics.ColorUtils
import coil3.ImageLoader
import coil3.SingletonImageLoader
import coil3.annotation.DelicateCoilApi
import coil3.asImage
import coil3.decode.DataSource
import coil3.request.SuccessResult
import coil3.test.FakeImageLoaderEngine
import java.net.URI

/**
 * Stands in for the server's artwork in screenshots: every URL gets its own abstract picture in
 * the right shape, the same on every run. Logos (`logo-...` ids) are a white wordmark of the id.
 */
@OptIn(DelicateCoilApi::class)
object FakeArtwork {
    fun install(context: Context) {
        val engine = FakeImageLoaderEngine.Builder()
            .default { chain -> SuccessResult(art(chain.request.data.toString()).asImage(), chain.request, DataSource.MEMORY) }
            .build()
        val loader = ImageLoader.Builder(context).components { add(engine) }.build()
        SingletonImageLoader.setUnsafe(loader)
    }

    private val cache = HashMap<String, Bitmap>()

    private fun art(url: String): Bitmap = cache.getOrPut(url) {
        val uri = URI(url)
        val id = uri.path.substringAfterLast('/')
        val size = uri.query.orEmpty().split('&').firstOrNull { it.startsWith("size=") }?.substringAfter('=')
        when {
            id.startsWith("logo-") -> logo(id.removePrefix("logo-"))
            size == "w342" -> picture(id, 342, 513)
            size == "w780" -> picture(id, 780, 439)
            else -> picture(id, 1280, 720)
        }
    }

    private fun picture(seed: String, width: Int, height: Int): Bitmap {
        val hue = (seed.hashCode().toLong() and 0xffff).toFloat() % 360f
        val bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        val top = ColorUtils.HSLToColor(floatArrayOf(hue, 0.55f, 0.42f))
        val bottom = ColorUtils.HSLToColor(floatArrayOf((hue + 50f) % 360f, 0.6f, 0.16f))
        val paint = Paint(Paint.ANTI_ALIAS_FLAG)
        paint.shader = LinearGradient(0f, 0f, width * 0.4f, height.toFloat(), top, bottom, Shader.TileMode.CLAMP)
        canvas.drawRect(0f, 0f, width.toFloat(), height.toFloat(), paint)
        paint.shader = null
        paint.color = ColorUtils.HSLToColor(floatArrayOf((hue + 180f) % 360f, 0.7f, 0.6f))
        paint.alpha = 70
        canvas.drawCircle(width * 0.7f, height * 0.35f, minOf(width, height) * 0.32f, paint)
        paint.color = 0xFF000000.toInt()
        paint.alpha = 60
        canvas.drawCircle(width * 0.25f, height * 0.8f, minOf(width, height) * 0.45f, paint)
        return bitmap
    }

    private fun logo(slug: String): Bitmap {
        val text = slug.replace('-', ' ').uppercase()
        val paint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
            color = 0xFFFFFFFF.toInt()
            textSize = 96f
            typeface = Typeface.create(Typeface.SERIF, Typeface.BOLD)
        }
        val width = paint.measureText(text).toInt() + 16
        val bitmap = Bitmap.createBitmap(width, 130, Bitmap.Config.ARGB_8888)
        Canvas(bitmap).drawText(text, 8f, 100f, paint)
        return bitmap
    }
}
