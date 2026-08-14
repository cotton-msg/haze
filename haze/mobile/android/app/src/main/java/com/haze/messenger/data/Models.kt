package com.haze.messenger.data

import kotlinx.serialization.Serializable

@Serializable
data class ApiResponse<T>(
    val error: Boolean = false,
    val data: T? = null,
    val message: String? = null,
)

@Serializable
data class TokenPair(
    val access_token: String,
    val refresh_token: String,
)

@Serializable
data class User(
    val id: String,
    val username: String,
    val display_name: String = "",
    val avatar_url: String = "",
    val bio: String = "",
    val role: String = "user",
    val is_premium: Boolean = false,
)

@Serializable
data class Chat(
    val id: String,
    val type: String,
    val title: String,
    val avatar: String = "",
    val last_message: String? = null,
    val unread_count: Int = 0,
    val updated_at: String = "",
)

@Serializable
data class Message(
    val id: String,
    val chat_id: String,
    val sender_id: String,
    val content: String,
    val type: String = "text",
    val reply_to: String? = null,
    val status: String = "sent",
    val created_at: String = "",
)

@Serializable
data class LoginResult(
    val tokens: TokenPair,
    val user: User,
)
