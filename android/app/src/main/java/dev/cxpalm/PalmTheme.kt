package dev.cxpalm

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.platform.LocalView
import androidx.activity.compose.LocalActivity
import androidx.core.view.WindowCompat
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
    primary = Color(0xFF9AC4FF), onPrimary = Color(0xFF0C2D54), primaryContainer = Color(0xFF233E5B), onPrimaryContainer = Color(0xFFDBEBFF),
    secondary = Color(0xFFB8C9DF), secondaryContainer = Color(0xFF2B3C50), onSecondaryContainer = Color(0xFFE2ECF8),
    background = Color(0xFF18212B), onBackground = Color(0xFFE8EEF6), surface = Color(0xFF202C39), onSurface = Color(0xFFE8EEF6),
    surfaceVariant = Color(0xFF2B394A), onSurfaceVariant = Color(0xFFB4C3D5), outline = Color(0xFF91A3BA), outlineVariant = Color(0xFF405167),
    surfaceContainerLowest = Color(0xFF141D27), surfaceContainerLow = Color(0xFF1C2733), surfaceContainer = Color(0xFF243140),
    surfaceContainerHigh = Color(0xFF2B394A), surfaceContainerHighest = Color(0xFF34465A),
    error = Color(0xFFFF9D92), errorContainer = Color(0xFF3A221E), onErrorContainer = Color(0xFFFFDAD3))

private val PalmShapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp), small = RoundedCornerShape(8.dp), medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(16.dp), extraLarge = RoundedCornerShape(24.dp))

@Composable fun CxpalmTheme(content: @Composable () -> Unit) {
    val dark = appearanceIsDark(AppAppearance.mode, isSystemInDarkTheme())
    val view = LocalView.current
    val activity = LocalActivity.current
    SideEffect {
        activity?.window?.let { window -> WindowCompat.getInsetsController(window, view).apply {
            isAppearanceLightStatusBars = !dark
            isAppearanceLightNavigationBars = !dark
        } }
    }
    MaterialTheme(colorScheme = if (dark) DarkColors else LightColors, shapes = PalmShapes,
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
