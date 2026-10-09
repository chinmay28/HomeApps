package io.github.chinmay28.homeapps

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.drawable.GradientDrawable
import android.text.TextUtils
import android.view.Gravity
import android.view.View
import android.widget.FrameLayout
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.TextView
import androidx.core.content.ContextCompat
import io.github.chinmay28.homeapps.core.LaunchedApp

/**
 * What the bubble opens: the app's controls and every running app, over a
 * dimmed copy of the app it came from. Tapping the dim area goes back.
 */
@SuppressLint("ViewConstructor")
class SwitcherView(context: Context, private val listener: Listener) : FrameLayout(context) {
    interface Listener {
        fun onSwitcherHome()
        fun onSwitcherReload()
        fun onSwitcherBrowser()
        fun onSwitcherClose()
        fun onSwitcherPick(app: LaunchedApp)
    }

    private val density = resources.displayMetrics.density
    private fun dp(v: Int) = (v * density).toInt()
    private fun color(id: Int) = ContextCompat.getColor(context, id)

    private val panel = LinearLayout(context)
    private val runningRow = LinearLayout(context)

    val isShowing: Boolean get() = visibility == VISIBLE

    init {
        visibility = GONE
        setBackgroundColor(color(R.color.scrim))
        isClickable = true
        setOnClickListener { hide() }

        panel.orientation = LinearLayout.VERTICAL
        panel.isClickable = true // taps inside the panel must not dismiss it
        panel.background = GradientDrawable().apply {
            setColor(color(R.color.surface))
            val r = dp(24).toFloat()
            cornerRadii = floatArrayOf(r, r, r, r, 0f, 0f, 0f, 0f)
        }
        panel.setPadding(dp(16), dp(12), dp(16), dp(20))
        addView(panel, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT, Gravity.BOTTOM))

        panel.addView(View(context).apply {
            background = GradientDrawable().apply {
                setColor(color(R.color.line))
                cornerRadius = dp(3).toFloat()
            }
        }, LinearLayout.LayoutParams(dp(40), dp(5)).apply {
            gravity = Gravity.CENTER_HORIZONTAL
            bottomMargin = dp(16)
        })

        val actions = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL }
        actions.addView(action(R.drawable.ic_home, R.string.action_home) { listener.onSwitcherHome() }, weighted())
        actions.addView(action(R.drawable.ic_reload, R.string.action_reload) { listener.onSwitcherReload() }, weighted())
        actions.addView(action(R.drawable.ic_browser, R.string.action_browser) { listener.onSwitcherBrowser() }, weighted())
        actions.addView(action(R.drawable.ic_close, R.string.action_close) { listener.onSwitcherClose() }, weighted())
        panel.addView(actions)

        panel.addView(TextView(context).apply {
            setText(R.string.switcher_running)
            isAllCaps = true
            letterSpacing = 0.08f
            textSize = 12f
            setTextColor(color(R.color.muted))
        }, LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply {
            topMargin = dp(18)
            bottomMargin = dp(10)
        })

        runningRow.orientation = LinearLayout.HORIZONTAL
        panel.addView(HorizontalScrollView(context).apply {
            isHorizontalScrollBarEnabled = false
            addView(runningRow)
        })

        panel.addView(TextView(context).apply {
            setText(R.string.switcher_all_apps)
            gravity = Gravity.CENTER
            textSize = 15f
            setTypeface(typeface, android.graphics.Typeface.BOLD)
            setTextColor(color(R.color.on_accent))
            background = GradientDrawable().apply {
                setColor(color(R.color.accent))
                cornerRadius = dp(14).toFloat()
            }
            isClickable = true
            setOnClickListener { listener.onSwitcherHome() }
        }, LinearLayout.LayoutParams(LayoutParams.MATCH_PARENT, dp(48)).apply { topMargin = dp(18) })
    }

    /** Room for the gesture bar under the panel. */
    fun setBottomInset(px: Int) {
        panel.setPadding(dp(16), dp(12), dp(16), dp(20) + px)
    }

    fun show(running: List<LaunchedApp>, current: LaunchedApp?, icons: IconLoader) {
        runningRow.removeAllViews()
        for (app in running) runningRow.addView(chip(app, app.id == current?.id, icons))
        visibility = VISIBLE
        alpha = 0f
        panel.translationY = dp(80).toFloat()
        animate().alpha(1f).setDuration(150).start()
        panel.animate().translationY(0f).setDuration(200).start()
    }

    fun hide() {
        if (!isShowing) return
        animate().alpha(0f).setDuration(120).withEndAction { visibility = GONE }.start()
    }

    private fun weighted() = LinearLayout.LayoutParams(0, dp(64), 1f).apply {
        marginStart = dp(4)
        marginEnd = dp(4)
    }

    private fun action(icon: Int, label: Int, onClick: () -> Unit): View = TextView(context).apply {
        setText(label)
        gravity = Gravity.CENTER
        textSize = 12f
        setTextColor(color(R.color.text))
        setCompoundDrawablesWithIntrinsicBounds(0, icon, 0, 0)
        compoundDrawablePadding = dp(4)
        setPadding(dp(4), dp(8), dp(4), dp(8))
        background = GradientDrawable().apply {
            setColor(color(R.color.surface_2))
            setStroke(dp(1), color(R.color.line))
            cornerRadius = dp(14).toFloat()
        }
        isClickable = true
        setOnClickListener { onClick() }
    }

    private fun chip(app: LaunchedApp, isCurrent: Boolean, icons: IconLoader): View {
        val box = LinearLayout(context).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            isClickable = true
            contentDescription = app.name
            setOnClickListener { listener.onSwitcherPick(app) }
            setPadding(dp(4), dp(4), dp(4), dp(4))
        }
        val tile = AppTileView(context).apply { bind(app, icons) }
        box.addView(tile, LinearLayout.LayoutParams(dp(52), dp(52)))
        if (isCurrent) {
            tile.foreground = GradientDrawable().apply {
                setStroke(dp(2), color(R.color.text))
                cornerRadius = dp(15).toFloat()
            }
        }
        box.addView(TextView(context).apply {
            text = app.name
            textSize = 11f
            maxLines = 2
            ellipsize = TextUtils.TruncateAt.END
            gravity = Gravity.CENTER
            setTextColor(color(R.color.text))
        }, LinearLayout.LayoutParams(dp(64), LayoutParams.WRAP_CONTENT).apply { topMargin = dp(6) })
        box.layoutParams = LinearLayout.LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT).apply { marginEnd = dp(6) }
        return box
    }
}
