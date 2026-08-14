package com.haze.messenger

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import com.haze.messenger.data.ApiClient
import com.haze.messenger.data.WsClient
import com.haze.messenger.ui.HazeApp
import com.haze.messenger.ui.theme.TgAccent
import com.haze.messenger.ui.theme.TgBackground
import com.haze.messenger.ui.theme.TgPanel
import com.haze.messenger.ui.theme.TgText

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val api = ApiClient(applicationContext)
        val ws = WsClient(api)

        setContent {
            MaterialTheme(
                colorScheme = darkColorScheme(
                    primary = TgAccent,
                    secondary = TgAccent,
                    background = TgBackground,
                    surface = TgPanel,
                    onPrimary = TgText,
                    onBackground = TgText,
                    onSurface = TgText,
                )
            ) {
                HazeApp(api, ws)
            }
        }
    }
}
