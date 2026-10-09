package io.github.chinmay28.homeapps

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Outline
import android.graphics.Paint
import android.graphics.RectF
import android.view.MotionEvent
import android.view.View
import android.view.ViewConfiguration
import android.view.ViewOutlineProvider
import io.github.chinmay28.homeapps.core.BubblePosition
import io.github.chinmay28.homeapps.core.LaunchedApp
import kotlin.math.abs

/**
 * The one thing HomeApps draws over a full-screen app: a small floating
 * bubble showing the app's icon. Drag it anywhere — it snaps to the nearer
 * side — and tap it for the switcher. It fades back when left alone so it
 * does not compete with the app.
 *
 * A bubble rather than an edge swipe because Android already owns the edges
 * (back) and the bottom (home): a gesture there would fight the system.
 */
@SuppressLint("ViewConstructor")
class BubbleView(context: Context, private val onMoved: (onRight: Boolean, fraction: Float) -> Unit) : View(context) {
    private val density = resources.displayMetrics.density
    val sizePx = (52 * density).toInt()
    private val marginPx = (12 * density).toInt()
    private val tilePx = 34 * density
    private val touchSlop = ViewConfiguration.get(context).scaledTouchSlop

    private var app: LaunchedApp? = null
    private var icon: Bitmap? = null
    private val ring = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = 0xCC17171E.toInt() }
    private val stroke = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = density
        color = 0x24FFFFFF
    }
    private val tile = RectF()

    private var insetTop = 0
    private var insetBottom = 0
    private var downX = 0f
    private var downY = 0f
    private var startTx = 0f
    private var startTy = 0f
    private var dragging = false
    private val fade = Runnable { animate().alpha(IDLE_ALPHA).setDuration(400).start() }

    init {
        contentDescription = context.getString(R.string.bubble_label)
        isClickable = true
        elevation = 8 * density
        outlineProvider = object : ViewOutlineProvider() {
            override fun getOutline(view: View, outline: Outline) = outline.setOval(0, 0, view.width, view.height)
        }
    }

    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) = setMeasuredDimension(sizePx, sizePx)

    fun bind(app: LaunchedApp, icons: IconLoader) {
        this.app = app
        icon = null
        icons.load(app.iconUrl) { bmp ->
            if (this.app?.id == app.id) {
                icon = bmp
                invalidate()
            }
        }
        wake()
        invalidate()
    }

    /** Space a camera cutout or gesture bar takes, to keep the bubble clear of. */
    fun setInsets(top: Int, bottom: Int) {
        insetTop = top
        insetBottom = bottom
    }

    /** Puts the bubble where it was last left. Call once the parent is laid out. */
    fun place(onRight: Boolean, fraction: Float) {
        val parent = parent as? View ?: return
        val usable = parent.height - insetTop - insetBottom
        val y = insetTop + BubblePosition.fromFraction(fraction, usable, sizePx)
        val x = if (onRight) parent.width else 0
        val p = BubblePosition.snap(x, y, parent.width, parent.height, sizePx, marginPx, insetTop, insetBottom)
        translationX = p.x.toFloat()
        translationY = p.y.toFloat()
    }

    fun wake() {
        removeCallbacks(fade)
        animate().cancel()
        alpha = 1f
        postDelayed(fade, IDLE_DELAY_MS)
    }

    override fun onDraw(canvas: Canvas) {
        val r = width / 2f
        canvas.drawCircle(r, r, r, ring)
        canvas.drawCircle(r, r, r - stroke.strokeWidth / 2, stroke)
        val a = app ?: return
        val inset = (width - tilePx) / 2
        tile.set(inset, inset, inset + tilePx, inset + tilePx)
        TilePainter.draw(canvas, tile, tilePx * 0.29f, a, icon)
    }

    @SuppressLint("ClickableViewAccessibility") // performClick is called on a tap
    override fun onTouchEvent(event: MotionEvent): Boolean {
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                removeCallbacks(fade)
                animate().cancel()
                alpha = 1f
                downX = event.rawX
                downY = event.rawY
                startTx = translationX
                startTy = translationY
                dragging = false
            }
            MotionEvent.ACTION_MOVE -> {
                val dx = event.rawX - downX
                val dy = event.rawY - downY
                if (!dragging && (abs(dx) > touchSlop || abs(dy) > touchSlop)) dragging = true
                if (dragging) {
                    translationX = startTx + dx
                    translationY = startTy + dy
                }
            }
            MotionEvent.ACTION_UP -> {
                if (dragging) settle() else performClick()
                postDelayed(fade, IDLE_DELAY_MS)
            }
            MotionEvent.ACTION_CANCEL -> {
                if (dragging) settle()
                postDelayed(fade, IDLE_DELAY_MS)
            }
        }
        return true
    }

    override fun performClick(): Boolean = super.performClick()

    private fun settle() {
        val parent = parent as? View ?: return
        val p = BubblePosition.snap(
            translationX.toInt(), translationY.toInt(), parent.width, parent.height, sizePx, marginPx, insetTop, insetBottom,
        )
        animate().translationX(p.x.toFloat()).translationY(p.y.toFloat()).setDuration(180).start()
        val onRight = p.x > parent.width / 2
        val usable = parent.height - insetTop - insetBottom
        onMoved(onRight, BubblePosition.toFraction(p.y - insetTop, usable, sizePx))
    }

    private companion object {
        const val IDLE_ALPHA = 0.45f
        const val IDLE_DELAY_MS = 3_000L
    }
}
