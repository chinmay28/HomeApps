package io.github.chinmay28.homeapps

import android.content.Context
import androidx.core.content.edit
import io.github.chinmay28.homeapps.core.ServerList

/** The shell's only state: the servers it knows and which one to show, and where the bubble rests. */
class Prefs(context: Context) {
    private val sp = context.getSharedPreferences("homeapps", Context.MODE_PRIVATE)

    /** Reads the single address older builds kept under [KEY_SERVER] until the first write. */
    var servers: ServerList
        get() = ServerList.decode(sp.getString(KEY_SERVERS, null), legacy = sp.getString(KEY_SERVER, null))
        set(value) = sp.edit {
            putString(KEY_SERVERS, value.encode())
            remove(KEY_SERVER)
        }

    /** The server the launcher shows, or null while none is picked. */
    val serverUrl: String? get() = servers.active

    var bubbleOnRight: Boolean
        get() = sp.getBoolean(KEY_BUBBLE_RIGHT, true)
        set(value) = sp.edit { putBoolean(KEY_BUBBLE_RIGHT, value) }

    /** 0 = top, 1 = bottom of the space the bubble can use. */
    var bubbleY: Float
        get() = sp.getFloat(KEY_BUBBLE_Y, 0.62f)
        set(value) = sp.edit { putFloat(KEY_BUBBLE_Y, value) }

    private companion object {
        const val KEY_SERVER = "server_url" // one address, before ServerList
        const val KEY_SERVERS = "servers"
        const val KEY_BUBBLE_RIGHT = "bubble_right"
        const val KEY_BUBBLE_Y = "bubble_y"
    }
}
