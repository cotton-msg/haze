package com.haze.messenger.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Logout
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.haze.messenger.data.ApiClient
import com.haze.messenger.data.Chat
import com.haze.messenger.data.WsClient
import com.haze.messenger.data.WsEvent
import com.haze.messenger.ui.theme.TgAccent
import com.haze.messenger.ui.theme.TgBackground
import com.haze.messenger.ui.theme.TgDivider
import com.haze.messenger.ui.theme.TgHeader
import com.haze.messenger.ui.theme.TgPanel
import com.haze.messenger.ui.theme.TgText
import com.haze.messenger.ui.theme.TgTextSecondary

@Composable
fun ChatListScreen(
    api: ApiClient,
    ws: WsClient,
    onOpenChat: (String) -> Unit,
    onLogout: () -> Unit,
) {
    var chats by remember { mutableStateOf<List<Chat>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            chats = api.listChats()
        } catch (e: Exception) {
            error = e.message ?: "Не удалось загрузить чаты"
        } finally {
            loading = false
        }
    }

    LaunchedEffect(Unit) {
        ws.events.collect { event ->
            if (event.type == "new_message") {
                val msg = event.payload["message"]?.toString()
                val chatId = event.payload["chat_id"]?.toString()
                if (chatId != null) {
                    val chat = chats.find { it.id == chatId }
                    chats = chats.map {
                        if (it.id == chatId) it.copy(last_message = msg, updated_at = "now")
                        else it
                    }
                }
            }
        }
    }

    Column(modifier = Modifier.fillMaxSize()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(56.dp)
                .padding(horizontal = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "Haze",
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
                color = TgText,
                modifier = Modifier.weight(1f),
            )
            IconButton(onClick = onLogout) {
                Icon(Icons.Default.Logout, contentDescription = "Выйти", tint = TgTextSecondary)
            }
        }

        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = TgAccent)
            }
            error != null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(error ?: "", color = TgTextSecondary)
            }
            else -> LazyColumn(modifier = Modifier.fillMaxSize()) {
                items(chats, key = { it.id }) { chat ->
                    ChatRow(chat, onClick = { onOpenChat(chat.id) })
                }
            }
        }
    }
}

@Composable
private fun ChatRow(chat: Chat, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        AvatarBadge(chat.title)
        Spacer(Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                chat.title,
                fontSize = 16.sp,
                fontWeight = FontWeight.Medium,
                color = TgText,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.height(2.dp))
            Text(
                chat.last_message ?: "Нет сообщений",
                fontSize = 14.sp,
                color = TgTextSecondary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        if (chat.unread_count > 0) {
            Box(
                modifier = Modifier
                    .padding(start = 8.dp)
                    .size(24.dp)
                    .clip(CircleShape)
                    .background(TgAccent),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    "${chat.unread_count}",
                    fontSize = 13.sp,
                    color = Color.White,
                )
            }
        }
    }
    HorizontalDivider(
        modifier = Modifier.padding(start = 60.dp),
        thickness = 0.5.dp,
        color = TgDivider,
    )
}

@Composable
fun AvatarBadge(title: String, size: Int = 52) {
    val initial = title.take(1).uppercase()
    Box(
        modifier = Modifier
            .size(size.dp)
            .clip(RoundedCornerShape(16.dp))
            .background(TgPanel),
        contentAlignment = Alignment.Center,
    ) {
        Text(initial, fontSize = (size * 0.4).sp, color = TgAccent, fontWeight = FontWeight.Bold)
    }
}
