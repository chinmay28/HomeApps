package io.github.chinmay28.homeapps

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.text.InputType
import android.view.Gravity
import android.view.inputmethod.EditorInfo
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.core.content.ContextCompat

/**
 * First run, and "Switch HomeApps server": a field for a new server address
 * and, below it, the servers already saved. Tapping a saved one switches to
 * it; its × forgets it. The activity checks every address and calls
 * [showError] or moves on.
 */
@SuppressLint("ViewConstructor")
class SetupView(
    context: Context,
    prefill: String?,
    error: String?,
    private val saved: List<String>,
    private val active: String?,
    private val onConnect: (String) -> Unit,
    private val onForget: (String) -> Unit,
) : LinearLayout(context) {
    private val density = resources.displayMetrics.density
    private fun dp(v: Int) = (v * density).toInt()
    private fun color(id: Int) = ContextCompat.getColor(context, id)

    private val field = EditText(context)
    private val errorText = TextView(context)
    private val button = TextView(context)

    init {
        orientation = VERTICAL
        gravity = Gravity.CENTER_VERTICAL
        setBackgroundColor(color(R.color.bg))
        isClickable = true // nothing behind it is reachable while it shows
        val side = dp(24)
        setPadding(side, side, side, side)

        addView(TextView(context).apply {
            setText(if (saved.isEmpty()) R.string.setup_title else R.string.servers_title)
            textSize = 26f
            setTypeface(typeface, Typeface.BOLD)
            setTextColor(color(R.color.text))
        })
        addView(TextView(context).apply {
            setText(if (saved.isEmpty()) R.string.setup_body else R.string.servers_body)
            textSize = 15f
            setTextColor(color(R.color.muted))
        }, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply { topMargin = dp(8) })

        field.apply {
            setHint(R.string.setup_hint)
            setText(prefill.orEmpty().removeSuffix("/"))
            textSize = 16f
            isSingleLine = true
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            imeOptions = EditorInfo.IME_ACTION_GO
            setTextColor(color(R.color.text))
            setHintTextColor(color(R.color.muted))
            setPadding(dp(14), 0, dp(14), 0)
            background = GradientDrawable().apply {
                setColor(color(R.color.surface))
                setStroke(dp(1), color(R.color.line))
                cornerRadius = dp(12).toFloat()
            }
            setOnEditorActionListener { _, action, _ ->
                if (action == EditorInfo.IME_ACTION_GO) submit()
                action == EditorInfo.IME_ACTION_GO
            }
        }
        addView(field, LayoutParams(LayoutParams.MATCH_PARENT, dp(52)).apply { topMargin = dp(24) })

        errorText.apply {
            textSize = 14f
            setTextColor(color(R.color.danger))
        }
        addView(errorText, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply { topMargin = dp(10) })
        showError(error)

        button.apply {
            setText(R.string.setup_connect)
            gravity = Gravity.CENTER
            textSize = 16f
            setTypeface(typeface, Typeface.BOLD)
            setTextColor(color(R.color.on_accent))
            background = GradientDrawable().apply {
                setColor(color(R.color.accent))
                cornerRadius = dp(14).toFloat()
            }
            isClickable = true
            setOnClickListener { submit() }
        }
        addView(button, LayoutParams(LayoutParams.MATCH_PARENT, dp(52)).apply { topMargin = dp(16) })

        // Last, so a long list takes only the room left and scrolls.
        if (saved.isNotEmpty()) {
            addView(TextView(context).apply {
                setText(R.string.servers_saved)
                textSize = 13f
                setTypeface(typeface, Typeface.BOLD)
                setTextColor(color(R.color.muted))
            }, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT).apply { topMargin = dp(28) })
            val rows = LinearLayout(context).apply { orientation = VERTICAL }
            for (server in saved) rows.addView(row(server), LayoutParams(LayoutParams.MATCH_PARENT, dp(56)).apply { topMargin = dp(8) })
            addView(ScrollView(context).apply { addView(rows) }, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT))
        }
    }

    /** One saved server: tap to switch to it, × to forget it. */
    private fun row(server: String): LinearLayout = LinearLayout(context).apply {
        orientation = HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setPadding(dp(14), 0, dp(4), 0)
        background = GradientDrawable().apply {
            setColor(color(R.color.surface))
            setStroke(dp(1), color(if (server == active) R.color.accent else R.color.line))
            cornerRadius = dp(12).toFloat()
        }
        isClickable = true
        contentDescription = context.getString(R.string.servers_switch_to, display(server))
        setOnClickListener { if (button.isEnabled) onConnect(server) }

        addView(TextView(context).apply {
            text = display(server)
            textSize = 16f
            isSingleLine = true
            setTextColor(color(R.color.text))
        }, LayoutParams(0, LayoutParams.WRAP_CONTENT, 1f))
        if (server == active) {
            addView(TextView(context).apply {
                setText(R.string.servers_current)
                textSize = 12f
                setTypeface(typeface, Typeface.BOLD)
                setTextColor(color(R.color.accent))
            })
        }
        addView(TextView(context).apply {
            setText(R.string.servers_forget_mark)
            textSize = 22f
            gravity = Gravity.CENTER
            setTextColor(color(R.color.muted))
            contentDescription = context.getString(R.string.servers_forget, display(server))
            isClickable = true
            setOnClickListener { if (button.isEnabled) onForget(server) }
        }, LayoutParams(dp(48), dp(48)))
    }

    /** "http://pi5.local:8686/" reads as "pi5.local:8686"; https stays spelled out. */
    private fun display(server: String) = server.removePrefix("http://").removeSuffix("/")

    fun showError(message: String?) {
        errorText.text = message.orEmpty()
        errorText.visibility = if (message.isNullOrEmpty()) GONE else VISIBLE
    }

    fun setBusy(busy: Boolean) {
        button.isEnabled = !busy
        field.isEnabled = !busy
        button.setText(if (busy) R.string.setup_checking else R.string.setup_connect)
    }

    private fun submit() {
        if (button.isEnabled) onConnect(field.text.toString())
    }
}
