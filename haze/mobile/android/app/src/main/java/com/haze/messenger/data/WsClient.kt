package com.haze.messenger.data

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener

data class WsEvent(
    val type: String,
    val payload: kotlinx.serialization.json.JsonObject,
)

class WsClient(
    private val api: ApiClient,
    private val okHttp: OkHttpClient = OkHttpClient(),
) {
    private var socket: WebSocket? = null
    private val _events = MutableSharedFlow<WsEvent>(extraBufferCapacity = 64)
    val events: SharedFlow<WsEvent> = _events.asSharedFlow()

    fun connect() {
        val token = api.getAccessToken() ?: return
        val req = Request.Builder()
            .url("${api.wsUrl()}?token=$token")
            .build()
        socket = okHttp.newWebSocket(req, object : WebSocketListener() {
            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    val json = Json.parseToJsonElement(text).jsonObject
                    val type = json["type"]?.jsonPrimitive?.content ?: return
                    val payload = json["payload"]?.jsonObject ?: buildJsonObject {}
                    _events.tryEmit(WsEvent(type, payload))
                } catch (_: Exception) {
                }
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                // auto-reconnect handled by caller
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                _events.tryEmit(WsEvent("ws_disconnected", buildJsonObject {}))
            }
        })
    }

    fun disconnect() {
        socket?.close(1000, "bye")
        socket = null
    }
}
