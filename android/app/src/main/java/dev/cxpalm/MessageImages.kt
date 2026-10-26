package dev.cxpalm

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.rememberTransformableState
import androidx.compose.foundation.gestures.transformable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties

data class MessageImage(val id: String,val name: String)

internal fun imageSampleSize(width: Int,height: Int,limit: Int): Int {
    require(width>0&&height>0&&width.toLong()*height<=40_000_000)
    var sample=1;while(maxOf(width,height)/sample>limit)sample*=2;return sample
}
private fun decodeImage(bytes: ByteArray,limit: Int): Bitmap {
    val bounds=BitmapFactory.Options().apply {inJustDecodeBounds=true};BitmapFactory.decodeByteArray(bytes,0,bytes.size,bounds)
    val options=BitmapFactory.Options().apply {inSampleSize=imageSampleSize(bounds.outWidth,bounds.outHeight,limit)}
    return BitmapFactory.decodeByteArray(bytes,0,bytes.size,options)?:error("Image decode failed")
}

@Composable fun MessageImages(message: Message,load: suspend (Message,MessageImage)->ByteArray) {
    for(image in message.images) key(message.id,image.id) {
        var expanded by remember {mutableStateOf(false)}
        var retry by remember {mutableIntStateOf(0)}
        var bitmap by remember {mutableStateOf<Bitmap?>(null)}
        var failed by remember {mutableStateOf(false)}
        LaunchedEffect(message.id,image.id,retry) {
            failed=false
            try {bitmap=kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {decodeImage(load(message,image),1024)}}
            catch(cancelled:kotlinx.coroutines.CancellationException){throw cancelled}
            catch(_:Exception){failed=true}
        }
        Column(Modifier.fillMaxWidth(),verticalArrangement=Arrangement.spacedBy(6.dp)) {
            bitmap?.let {preview ->
                Image(preview.asImageBitmap(),contentDescription=image.name,contentScale=ContentScale.Fit,modifier=Modifier.fillMaxWidth().heightIn(max=320.dp).clickable {expanded=true})
                TextButton(onClick={expanded=true}) {Text(tr("查看图片"))}
            } ?: if(failed) TextButton(onClick={retry++}) {Text(tr("图片读取失败 · 点击重试"))} else Text(tr("正在读取图片…"),style=MaterialTheme.typography.bodySmall)
        }
        if(expanded) Dialog(onDismissRequest={expanded=false},properties=DialogProperties(usePlatformDefaultWidth=false)) {
            var full by remember {mutableStateOf<Bitmap?>(null)}
            var fullFailed by remember {mutableStateOf(false)}
            var zoom by remember {mutableFloatStateOf(1f)}
            var pan by remember {mutableStateOf(Offset.Zero)}
            LaunchedEffect(image.id) {
                try {full=kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {decodeImage(load(message,image),4096)}}
                catch(cancelled:kotlinx.coroutines.CancellationException){throw cancelled}
                catch(_:Exception){fullFailed=true}
            }
            Column(Modifier.fillMaxSize().background(Color.Black).systemBarsPadding()) {
                TextButton(onClick={expanded=false}) {Text(tr("关闭图片"),color=Color.White)}
                Box(Modifier.fillMaxSize(),contentAlignment=Alignment.Center) {
                    (full?:bitmap)?.let {shown -> Image(shown.asImageBitmap(),image.name,contentScale=ContentScale.Fit,modifier=Modifier.fillMaxSize().graphicsLayer {scaleX=zoom;scaleY=zoom;translationX=pan.x;translationY=pan.y}.transformable(rememberTransformableState {scale,offset,_ ->zoom=(zoom*scale).coerceIn(1f,8f);pan=if(zoom==1f)Offset.Zero else pan+offset}))}
                    if(fullFailed) Text(tr("原图暂不可用"),color=Color.White,modifier=Modifier.align(Alignment.BottomCenter).padding(16.dp))
                }
            }
        }
    }
}
