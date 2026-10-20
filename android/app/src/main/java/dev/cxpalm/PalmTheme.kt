package dev.cxpalm

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Shared with the Web client: neutral paper, white surfaces, one blue accent.
private val LightColors = lightColorScheme(
    primary = Color(0xFF2457F5), onPrimary = Color.White, primaryContainer = Color(0xFFE9EFFF), onPrimaryContainer = Color(0xFF14286B),
    secondary = Color(0xFF55627A), secondaryContainer = Color(0xFFECF1FF), onSecondaryContainer = Color(0xFF1F2F55),
    background = Color(0xFFF6F7FA), onBackground = Color(0xFF1B2230), surface = Color(0xFFFFFFFF), onSurface = Color(0xFF1B2230),
    surfaceVariant = Color(0xFFEEF1F6), onSurfaceVariant = Color(0xFF667185), outline = Color(0xFF8A94A6), outlineVariant = Color(0xFFE2E6EE),
    surfaceContainerLowest = Color.White, surfaceContainerLow = Color(0xFFFAFBFC), surfaceContainer = Color(0xFFF2F4F8),
    surfaceContainerHigh = Color(0xFFEEF1F6), surfaceContainerHighest = Color(0xFFE6EAF1),
    error = Color(0xFFB4342B), errorContainer = Color(0xFFFFF0EC), onErrorContainer = Color(0xFF7F251E))
private val DarkColors = darkColorScheme(
    primary = Color(0xFFB6C8FF), onPrimary = Color(0xFF10275F), primaryContainer = Color(0xFF1F2B47), onPrimaryContainer = Color(0xFFDFE7FF),
    secondary = Color(0xFFB8C4DA), secondaryContainer = Color(0xFF1E2940), onSecondaryContainer = Color(0xFFDCE6FA),
    background = Color(0xFF111419), onBackground = Color(0xFFE6EAF1), surface = Color(0xFF181C23), onSurface = Color(0xFFE6EAF1),
    surfaceVariant = Color(0xFF20252E), onSurfaceVariant = Color(0xFF98A3B5), outline = Color(0xFF6E7889), outlineVariant = Color(0xFF2A303B),
    surfaceContainerLowest = Color(0xFF0C0F13), surfaceContainerLow = Color(0xFF15191F), surfaceContainer = Color(0xFF1B2028),
    surfaceContainerHigh = Color(0xFF20252E), surfaceContainerHighest = Color(0xFF272D37),
    error = Color(0xFFFF9D92), errorContainer = Color(0xFF3A221E), onErrorContainer = Color(0xFFFFDAD3))

private val PalmShapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp), small = RoundedCornerShape(8.dp), medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(16.dp), extraLarge = RoundedCornerShape(24.dp))

@Composable fun CxpalmTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = if (isSystemInDarkTheme()) DarkColors else LightColors, shapes = PalmShapes,
        typography = Typography(
            headlineLarge = TextStyle(fontSize = 30.sp, lineHeight = 40.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.3).sp),
            titleLarge = TextStyle(fontSize = 22.sp, lineHeight = 30.sp, fontWeight = FontWeight.SemiBold),
            titleMedium = TextStyle(fontSize = 16.sp, lineHeight = 24.sp, fontWeight = FontWeight.SemiBold),
            titleSmall = TextStyle(fontSize = 15.sp, lineHeight = 22.sp, fontWeight = FontWeight.SemiBold),
            bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 26.sp),
            bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 22.sp),
            bodySmall = TextStyle(fontSize = 12.sp, lineHeight = 18.sp),
            labelLarge = TextStyle(fontSize = 14.sp, lineHeight = 20.sp, fontWeight = FontWeight.Medium),
            labelMedium = TextStyle(fontSize = 13.sp, lineHeight = 18.sp, fontWeight = FontWeight.SemiBold),
            labelSmall = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 11.sp, lineHeight = 16.sp)), content = content)
}
