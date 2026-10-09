package io.github.chinmay28.homeapps

import android.content.Context
import androidx.core.content.edit

/** The shell's only state: which server to show, and where the bubble rests. */
class Prefs(context: Context) {
    private val sp = context.getSharedPreferences("homeapps", Context.MODE_PRIVATE)

    var serverUrl: String?
        get() = sp.getString(KEY_SERVER, null)
        set(value) = sp.edit { putString(KEY_SERVER, value) }

    var bubbleOnRight: Boolean
        get() = sp.getBoolean(KEY_BUBBLE_RIGHT, true)
        set(value) = sp.edit { putBoolean(KEY_BUBBLE_RIGHT, value) }

    /** 0 = top, 1 = bottom of the space the bubble can use. */
    var bubbleY: Float
        get() = sp.getFloat(KEY_BUBBLE_Y, 0.62f)
        set(value) = sp.edit { putFloat(KEY_BUBBLE_Y, value) }

    private companion object {
        const val KEY_SERVER = "server_url"
        const val KEY_BUBBLE_RIGHT = "bubble_right"
        const val KEY_BUBBLE_Y = "bubble_y"
    }
}
