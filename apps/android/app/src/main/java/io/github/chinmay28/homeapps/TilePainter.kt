package io.github.chinmay28.homeapps

import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RectF
import android.graphics.Typeface
import io.github.chinmay28.homeapps.core.LaunchedApp
import io.github.chinmay28.homeapps.core.Tile

/**
 * Draws an app tile — its icon, or its monogram on the web client's colours —
 * into a rounded rectangle. UI thread only (shared Paint and Path).
 */
object TilePainter {
    private const val SURFACE = 0xFF1E1E27.toInt()
    private val paint = Paint(Paint.ANTI_ALIAS_FLAG or Paint.FILTER_BITMAP_FLAG)
    private val text = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
        textAlign = Paint.Align.CENTER
    }
    private val clip = Path()

    fun draw(canvas: Canvas, rect: RectF, radius: Float, app: LaunchedApp, icon: Bitmap?) {
        val saved = canvas.save()
        clip.reset()
        clip.addRoundRect(rect, radius, radius, Path.Direction.CW)
        canvas.clipPath(clip)
        if (icon != null) {
            paint.color = app.color ?: SURFACE
            canvas.drawRect(rect, paint)
            canvas.drawBitmap(icon, null, rect, paint)
        } else {
            paint.color = Tile.background(app.name)
            canvas.drawRect(rect, paint)
            text.color = Tile.foreground(app.name)
            text.textSize = rect.height() * 0.36f
            val baseline = rect.centerY() - (text.descent() + text.ascent()) / 2
            canvas.drawText(app.monogram, rect.centerX(), baseline, text)
        }
        canvas.restoreToCount(saved)
    }
}
