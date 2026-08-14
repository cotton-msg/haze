package com.haze.messenger.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Send
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.haze.messenger.data.ApiClient
import com.haze.messenger.data.Message
import com.haze.messenger.data.WsClient
import com.haze.messenger.ui.theme.TgAccent
import com.haze.messenger.ui.theme.TgDivider
import com.haze.messenger.ui.theme.TgInput
import com.haze.messenger.ui.theme.TgOwnBubble
import com.haze.messenger.ui.theme.TgPeerBubble
import com.haze.messenger.ui.theme.TgText
import com.haze.messenger.ui.theme.TgTextSecondary
import kotlinx.coroutines.launch
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonPrimitive

@Composable
fun ChatScreen(
    api: ApiClient,
    ws: WsClient,
    chatId: String,
    onBack: () -> Unit,
) {
    var messages by remember { mutableStateOf<List<Message>>(emptyList()) }
    var input by remember { mutableStateOf("") }
    var loading by remember { mutableStateOf(true) }
    var myUserId by remember { mutableStateOf<String?>(null) }
    val listState = rememberLazyListState()
    val scope = rememberCoroutineScope()
    val wsEvents by ws.events.collectAsState(initial = null)

    LaunchedEffect(chatId) {
        try {
            messages = api.getMessages(chatId).sortedBy { it.created_at }
            myUserId = try {
                api.getMe().id
            } catch (_: Exception) {
                null
            }
        } catch (_: Exception) {
        } finally {
            loading = false
        }
        if (messages.isNotEmpty()) {
            listState.scrollToItem(messages.size - 1)
        }
    }

    LaunchedEffect(wsEvents) {
        val event = wsEvents ?: return@LaunchedEffect
        when (event.type) {
            "new_message" -> {
                val msgJson = event.payload["message"]?.toString()
                val chat = event.payload["chat_id"]?.jsonPrimitive?.content
                if (chat == chatId && msgJson != null) {
                    val message = Json { ignoreUnknownKeys = true }
                        .decodeFromString<Message>(msgJson)
                    if (messages.none { it.id == message.id }) {
                        messages = messages + message
                    }
                }
            }
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .imePadding()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(56.dp)
                .padding(horizontal = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Назад", tint = TgText)
            }
            Text("Чат", fontSize = 18.sp, fontWeight = FontWeight.SemiBold, color = TgText)
        }

        when {
            loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = TgAccent)
            }
            messages.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Сообщений пока нет", color = TgTextSecondary)
            }
            else -> LazyColumn(
                state = listState,
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 12.dp, vertical = 8.dp),
            ) {
                items(messages, key = { it.id }) { msg ->
                    MessageBubble(msg, isOwn = msg.sender_id == myUserId)
                }
            }
        }

        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            OutlinedTextField(
                value = input,
                onValueChange = {
                    input = it
                    if (it.length % 10 == 0) {
                        scope.launch { runCatching { api.sendTyping(chatId) } }
                    }
                },
                modifier = Modifier.weight(1f),
                placeholder = { Text("Сообщение", color = TgTextSecondary) },
                maxLines = 4,
                shape = RoundedCornerShape(16.dp),
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = TgInput,
                    unfocusedBorderColor = TgInput,
                    focusedTextColor = TgText,
                    unfocusedTextColor = TgText,
                    cursorColor = TgAccent,
                ),
            )
            SendButton(
                enabled = input.isNotBlank(),
                onClick = {
                    val text = input.trim()
                    if (text.isNotEmpty()) {
                        scope.launch {
                            runCatching { api.sendMessage(chatId, text) }
                            input = ""
                        }
                    }
                },
            )
        }
    }
}

@Composable
private fun MessageBubble(msg: Message, isOwn: Boolean) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 3.dp),
        horizontalArrangement = if (isOwn) Arrangement.End else Arrangement.Start,
    ) {
        Box(
            modifier = Modifier
                .widthIn(max = 300.dp)
                .background(
                    if (isOwn) TgOwnBubble else TgPeerBubble,
                    RoundedCornerShape(
                        topStart = 16.dp,
                        topEnd = 16.dp,
                        bottomStart = if (isOwn) 16.dp else 4.dp,
                        bottomEnd = if (isOwn) 4.dp else 16.dp,
                    ),
                )
                .padding(horizontal = 10.dp, vertical = 7.dp),
        ) {
            Column {
                if (msg.type != "text") {
                    Text(
                        when (msg.type) {
                            "voice" -> "🎤 Голосовое"
                            "file" -> "📎 Файл"
                            "image" -> "🖼 Фото"
                            else -> "Файл"
                        },
                        color = TgText,
                        fontSize = 14.sp,
                    )
                }
                Text(
                    msg.content,
                    color = TgText,
                    fontSize = 15.sp,
                )
                Spacer(Modifier.height(2.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        formatTime(msg.created_at),
                        color = TgTextSecondary,
                        fontSize = 11.sp,
                    )
                    if (isOwn) {
                        Spacer(Modifier.padding(horizontal = 1.dp))
                        Text(
                            when (msg.status) {
                                "read" -> "✓✓"
                                "delivered" -> "✓✓"
                                else -> "✓"
                            },
                            color = if (msg.status == "read") TgAccent else TgTextSecondary,
                            fontSize = 11.sp,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun SendButton(enabled: Boolean, onClick: () -> Unit) {
    IconButton(
        onClick = onClick,
        enabled = enabled,
    ) {
        Icon(
            imageVector = Icons.Default.Send,
            contentDescription = "Отправить",
            tint = if (enabled) TgAccent else TgTextSecondary,
        )
    }
}

private fun formatTime(iso: String): String {
    return try {
        val t = java.time.Instant.parse(iso)
        val dt = java.time.ZonedDateTime.ofInstant(t, java.time.ZoneId.systemDefault())
        "%02d:%02d".format(dt.hour, dt.minute)
    } catch (_: Exception) {
        ""
    }
}
