package io.github.chinmay28.homeapps

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.RectF
import android.view.View
import io.github.chinmay28.homeapps.core.LaunchedApp

/** One app's tile, for the switcher's row of running apps. */
class AppTileView(context: Context) : View(context) {
    private var app: LaunchedApp? = null
    private var icon: Bitmap? = null
    private val rect = RectF()

    fun bind(app: LaunchedApp, icons: IconLoader) {
        this.app = app
        icon = null
        contentDescription = app.name
        icons.load(app.iconUrl) { bmp ->
            if (this.app?.id == app.id) {
                icon = bmp
                invalidate()
            }
        }
        invalidate()
    }

    override fun onDraw(canvas: Canvas) {
        val a = app ?: return
        rect.set(0f, 0f, width.toFloat(), height.toFloat())
        TilePainter.draw(canvas, rect, width * 0.29f, a, icon)
    }
}
