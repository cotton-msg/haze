package com.haze.messenger.ui

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.haze.messenger.data.ApiClient
import com.haze.messenger.data.WsClient
import com.haze.messenger.ui.screens.ChatListScreen
import com.haze.messenger.ui.screens.ChatScreen
import com.haze.messenger.ui.screens.LoginScreen

enum class Screen {
    Login, ChatList, Chat
}

@Composable
fun HazeApp(api: ApiClient, ws: WsClient) {
    var screen by remember { mutableStateOf(if (api.getAccessToken() != null) Screen.ChatList else Screen.Login) }
    var activeChatId by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(screen) {
        if (screen != Screen.Login) {
            ws.connect()
        } else {
            ws.disconnect()
        }
    }

    when (screen) {
        Screen.Login -> LoginScreen(api, onLoggedIn = { screen = Screen.ChatList })
        Screen.ChatList -> ChatListScreen(
            api = api,
            ws = ws,
            onOpenChat = { id ->
                activeChatId = id
                screen = Screen.Chat
            },
            onLogout = {
                api.clearTokens()
                screen = Screen.Login
            },
        )
        Screen.Chat -> ChatScreen(
            api = api,
            ws = ws,
            chatId = activeChatId ?: "",
            onBack = { screen = Screen.ChatList },
        )
    }
}
