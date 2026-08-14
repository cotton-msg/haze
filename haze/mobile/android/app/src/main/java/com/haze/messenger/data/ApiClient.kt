package com.haze.messenger.data

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody

private val JSON_MEDIA: okhttp3.MediaType? = "application/json; charset=utf-8".toMediaTypeOrNull()

class ApiClient(context: Context) {

    private val prefs: SharedPreferences =
        context.getSharedPreferences("haze_auth", Context.MODE_PRIVATE)

    private val client = OkHttpClient.Builder()
        .connectTimeout(10, java.util.concurrent.TimeUnit.SECONDS)
        .readTimeout(20, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    val baseUrl: String
        get() = prefs.getString("server_url", "http://10.0.2.2:8080") ?: "http://10.0.2.2:8080"

    fun setServerUrl(url: String) {
        prefs.edit().putString("server_url", url).apply()
    }

    fun saveTokens(access: String, refresh: String) {
        prefs.edit()
            .putString("access_token", access)
            .putString("refresh_token", refresh)
            .apply()
    }

    fun getAccessToken(): String? = prefs.getString("access_token", null)

    fun clearTokens() {
        prefs.edit().remove("access_token").remove("refresh_token").apply()
    }

    fun wsUrl(): String {
        val base = baseUrl
        val wsBase = base.replace("http://", "ws://").replace("https://", "wss://")
        return "$wsBase/api/chat/ws"
    }

    suspend fun login(ssaCode: String): LoginResult = withContext(Dispatchers.IO) {
        val body = Json.encodeToString(mapOf("ssa_code" to ssaCode)).toRequestBody(JSON_MEDIA)
        val req = Request.Builder()
            .url("$baseUrl/api/auth/login")
            .post(body)
            .build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string() ?: ""
            if (!resp.isSuccessful) throw RuntimeException("login failed: ${resp.code} $text")
            parse<LoginResult>(text)
        }
    }

    suspend fun register(ssaCode: String, username: String, displayName: String): LoginResult =
        withContext(Dispatchers.IO) {
            val body = Json.encodeToString(
                mapOf(
                    "ssa_code" to ssaCode,
                    "username" to username,
                    "display_name" to displayName,
                )
            ).toRequestBody(JSON_MEDIA)
            val req = Request.Builder()
                .url("$baseUrl/api/auth/register")
                .post(body)
                .build()
            client.newCall(req).execute().use { resp ->
                val text = resp.body?.string() ?: ""
                if (!resp.isSuccessful) throw RuntimeException("register failed: ${resp.code} $text")
                parse<LoginResult>(text)
            }
        }

    suspend fun getMe(): User = get("/api/auth/me")

    suspend fun listChats(): List<Chat> = get("/api/chat/list")

    suspend fun getMessages(chatId: String, limit: Int = 50, offset: Int = 0): List<Message> =
        get("/api/chat/$chatId/messages?limit=$limit&offset=$offset")

    suspend fun sendMessage(chatId: String, content: String, type: String = "text"): Message =
        post("/api/chat/$chatId/message", mapOf("content" to content, "type" to type))

    suspend fun markRead(chatId: String, messageId: String) {
        postNoContent("/api/chat/$chatId/read", mapOf("message_id" to messageId))
    }

    suspend fun sendTyping(chatId: String) {
        postNoContent("/api/chat/$chatId/typing", emptyMap<String, String>())
    }

    private suspend inline fun <reified T> get(path: String): T = withContext(Dispatchers.IO) {
        val req = Request.Builder()
            .url("$baseUrl$path")
            .header("Authorization", "Bearer ${getAccessToken()}")
            .build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string() ?: ""
            if (!resp.isSuccessful) throw RuntimeException("GET $path failed: ${resp.code} $text")
            parse<T>(text)
        }
    }

    private suspend inline fun <reified T> post(path: String, body: Map<String, Any>): T =
        withContext(Dispatchers.IO) {
            val req = Request.Builder()
                .url("$baseUrl$path")
                .header("Authorization", "Bearer ${getAccessToken()}")
                .post(Json.encodeToString(body).toRequestBody(JSON_MEDIA))
                .build()
            client.newCall(req).execute().use { resp ->
                val text = resp.body?.string() ?: ""
                if (!resp.isSuccessful) throw RuntimeException("POST $path failed: ${resp.code} $text")
                parse<T>(text)
            }
        }

    private suspend fun postNoContent(path: String, body: Map<String, Any>) =
        withContext(Dispatchers.IO) {
            val req = Request.Builder()
                .url("$baseUrl$path")
                .header("Authorization", "Bearer ${getAccessToken()}")
                .post(Json.encodeToString(body).toRequestBody(JSON_MEDIA))
                .build()
            client.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) {
                    throw RuntimeException("POST $path failed: ${resp.code} ${resp.body?.string()}")
                }
            }
        }

    private inline fun <reified T> parse(text: String): T {
        val json = Json {
            ignoreUnknownKeys = true
        }
        val wrapper = json.decodeFromString<ApiResponse<T>>(text)
        if (wrapper.error) throw RuntimeException(wrapper.message ?: "request failed")
        @Suppress("UNCHECKED_CAST")
        return (wrapper.data as T)
    }
}
