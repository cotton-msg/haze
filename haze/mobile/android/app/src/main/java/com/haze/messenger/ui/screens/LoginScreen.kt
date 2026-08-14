package com.haze.messenger.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.haze.messenger.data.ApiClient
import com.haze.messenger.ui.theme.TgAccent
import com.haze.messenger.ui.theme.TgBackground
import com.haze.messenger.ui.theme.TgInput
import com.haze.messenger.ui.theme.TgText
import com.haze.messenger.ui.theme.TgTextSecondary
import kotlinx.coroutines.launch

@Composable
fun LoginScreen(api: ApiClient, onLoggedIn: () -> Unit) {
    var code by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            "Haze",
            fontSize = 34.sp,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.primary,
        )
        Spacer(Modifier.height(8.dp))
        Text(
            "Введите SSA-код из браузера",
            fontSize = 15.sp,
            color = TgTextSecondary,
            textAlign = TextAlign.Center,
        )
        Spacer(Modifier.height(24.dp))

        OutlinedTextField(
            value = code,
            onValueChange = { code = it },
            label = { Text("SSA-код") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = TgAccent,
                unfocusedBorderColor = TgInput,
                focusedTextColor = TgText,
                unfocusedTextColor = TgText,
                cursorColor = TgAccent,
            ),
        )

        error?.let {
            Spacer(Modifier.height(12.dp))
            Text(it, color = TgTextSecondary, fontSize = 13.sp)
        }

        Spacer(Modifier.height(20.dp))
        Button(
            onClick = {
                if (code.isBlank()) {
                    error = "Введите код"
                    return@Button
                }
                loading = true
                error = null
                scope.launch {
                    try {
                        val result = api.login(code)
                        api.saveTokens(result.tokens.access_token, result.tokens.refresh_token)
                        onLoggedIn()
                    } catch (e: Exception) {
                        error = e.message ?: "Ошибка входа"
                    } finally {
                        loading = false
                    }
                }
            },
            enabled = !loading,
            modifier = Modifier
                .fillMaxWidth()
                .height(48.dp),
            shape = RoundedCornerShape(12.dp),
            colors = ButtonDefaults.buttonColors(containerColor = TgAccent),
        ) {
            if (loading) {
                CircularProgressIndicator(
                    modifier = Modifier.size(22.dp),
                    color = TgBackground,
                    strokeWidth = 2.dp,
                )
            } else {
                Text("Войти", fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = TgBackground)
            }
        }
    }
}
