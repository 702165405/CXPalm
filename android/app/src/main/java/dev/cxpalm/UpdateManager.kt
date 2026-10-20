package dev.cxpalm

import android.content.Context
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import okhttp3.Call
import java.io.File
import java.io.IOException
import java.util.concurrent.CancellationException
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicLong

enum class UpdatePhase { IDLE, CHECKING, CURRENT, AVAILABLE, DOWNLOADING, READY, ERROR }
data class UpdateState(val phase: UpdatePhase = UpdatePhase.IDLE, val manifest: UpdateManifest? = null,
    val downloaded: Long = 0, val message: String? = null, val lastChecked: Long = 0)

class UpdateManager(context: Context) {
    private val app = context.applicationContext
    private val prefs = app.getSharedPreferences("cxpalm-updates", Context.MODE_PRIVATE)
    private val main = Handler(Looper.getMainLooper())
    private val worker = Executors.newSingleThreadExecutor()
    private val generation = AtomicLong()
    private val transport = UpdateTransport()
    private val directory = File(app.cacheDir, "updates").apply { mkdirs() }
    private val partial = File(directory, "incoming.apk.part")
    private val ready = File(directory, "cxpalm-update.apk")
    private val metadata = File(directory, "manifest.json")
    @Volatile private var call: Call? = null
    private val mutable = MutableStateFlow(UpdateState(lastChecked = prefs.getLong("last-check", 0)))
    val state = mutable.asStateFlow()

    init {
        val gen = generation.get()
        worker.execute {
            partial.delete()
            try {
                if (ready.exists() && metadata.exists() && System.currentTimeMillis() - ready.lastModified() in 0..UPDATE_CACHE_MS) {
                    val manifest = parseUpdateManifest(metadata.readText(), app.packageName)
                    validate(ready, manifest)
                    post(gen) { it.copy(phase = UpdatePhase.READY, manifest = manifest) }
                } else clearCache()
            } catch (_: Exception) { clearCache() }
        }
    }

    fun foreground() {
        if (mutable.value.phase == UpdatePhase.READY && System.currentTimeMillis() - ready.lastModified() !in 0..UPDATE_CACHE_MS) {
            generation.incrementAndGet(); mutable.value = mutable.value.copy(phase = UpdatePhase.IDLE, manifest = null)
            worker.execute { clearCache() }
        }
        check(false)
    }

    fun check(manual: Boolean = true) {
        val state = mutable.value
        if (state.phase in listOf(UpdatePhase.CHECKING, UpdatePhase.DOWNLOADING)) return
        val now = System.currentTimeMillis()
        if (!manual && !shouldAutoCheck(now, state.lastChecked)) return
        val gen = generation.incrementAndGet()
        if (!prefs.edit().putLong("last-check", now).commit()) {
            mutable.value = state.copy(phase = UpdatePhase.ERROR, message = "无法保存检查时间，请稍后重试")
            return
        }
        mutable.value = state.copy(phase = UpdatePhase.CHECKING, message = null, lastChecked = now)
        worker.execute {
            try {
                val manifest = transport.manifest(app.packageName) { call = it; active(gen) }
                active(gen)
                val newer = isNewerVersion(manifest.versionCode.toLong(), installed().versionCode)
                val cached = newer && ready.exists() && metadata.exists() &&
                    System.currentTimeMillis() - ready.lastModified() in 0..UPDATE_CACHE_MS &&
                    parseUpdateManifest(metadata.readText(), app.packageName) == manifest
                if (cached) validate(ready, manifest) else clearCache()
                post(gen) { it.copy(phase = if (cached) UpdatePhase.READY else if (newer) UpdatePhase.AVAILABLE else UpdatePhase.CURRENT,
                    manifest = if (newer) manifest else null, downloaded = 0, message = if (newer) null else "当前已是最新版本") }
            } catch (e: Exception) {
                if (state.phase == UpdatePhase.READY && ready.exists() && System.currentTimeMillis() - ready.lastModified() in 0..UPDATE_CACHE_MS)
                    post(gen) { it.copy(phase = UpdatePhase.READY, message = "检查更新失败；已校验的下载仍可安装") }
                else { if (state.phase == UpdatePhase.READY) clearCache(); fail(gen, e) }
            }
        }
    }

    fun download() {
        val state = mutable.value
        val manifest = state.manifest ?: return
        if (state.phase in listOf(UpdatePhase.CHECKING, UpdatePhase.DOWNLOADING, UpdatePhase.READY)) return
        val gen = generation.incrementAndGet()
        mutable.value = state.copy(phase = UpdatePhase.DOWNLOADING, downloaded = 0, message = null)
        worker.execute {
            try {
                clearCache()
                var lastProgress = 0L
                transport.download(manifest, partial, { call = it; active(gen) }, { active(gen) }) { count ->
                    val now = System.nanoTime()
                    if (now - lastProgress >= 100_000_000 || count == manifest.size) {
                        lastProgress = now; post(gen) { it.copy(downloaded = count) }
                    }
                }
                validate(partial, manifest); active(gen)
                check(partial.renameTo(ready)) { "无法保存更新缓存" }
                metadata.writeText(manifest.json()); active(gen)
                post(gen) {
                    localNotification(app, "updates", 3, "更新已下载", "CXPalm ${manifest.version} 已校验，点击进入设置安装。", updates = true)
                    it.copy(phase = UpdatePhase.READY, downloaded = manifest.size)
                }
            } catch (e: Exception) { clearCache(); fail(gen, e) }
        }
    }

    fun cancel() {
        if (mutable.value.phase != UpdatePhase.DOWNLOADING) return
        generation.incrementAndGet(); call?.cancel()
        mutable.value = mutable.value.copy(phase = UpdatePhase.AVAILABLE, downloaded = 0, message = "下载已取消；可随时重新下载")
        worker.execute { clearCache() }
    }

    fun installError(message: String) { mutable.value = mutable.value.copy(message = message) }

    // Revalidate immediately before granting the installer access, including after returning from settings.
    fun forInstall(result: (File?) -> Unit) {
        val state = mutable.value
        val manifest = state.manifest ?: return result(null)
        if (state.phase != UpdatePhase.READY) return result(null)
        val gen = generation.get()
        worker.execute {
            try {
                require(System.currentTimeMillis() - ready.lastModified() in 0..UPDATE_CACHE_MS) { "更新缓存已过期，请重新下载" }
                validate(ready, manifest); active(gen)
                main.post { if (generation.get() == gen) result(ready) else result(null) }
            } catch (e: Exception) { clearCache(); fail(gen, e); main.post { result(null) } }
        }
    }

    @Suppress("DEPRECATION")
    private fun identity(info: PackageInfo): ApkIdentity {
        val signatures = if (Build.VERSION.SDK_INT >= 28) info.signingInfo?.apkContentsSigners else info.signatures
        val code = if (Build.VERSION.SDK_INT >= 28) info.longVersionCode else info.versionCode.toLong()
        return ApkIdentity(info.packageName, info.versionName.orEmpty(), code, signatures.orEmpty().map { sha256(it.toByteArray()) }.toSet())
    }
    @Suppress("DEPRECATION")
    private fun flags(): Int = if (Build.VERSION.SDK_INT >= 28) PackageManager.GET_SIGNING_CERTIFICATES else PackageManager.GET_SIGNATURES
    @Suppress("DEPRECATION")
    private fun installed(): ApkIdentity = identity(app.packageManager.getPackageInfo(app.packageName, flags()))
    @Suppress("DEPRECATION")
    private fun validate(file: File, manifest: UpdateManifest) {
        verifyUpdateFile(file, manifest)
        val info = app.packageManager.getPackageArchiveInfo(file.absolutePath, flags()) ?: error("系统无法验证此 APK")
        verifyApkIdentity(manifest, installed(), identity(info))
    }
    private fun active(gen: Long) { if (generation.get() != gen) throw CancellationException() }
    private fun post(gen: Long, change: (UpdateState) -> UpdateState) { main.post { if (generation.get() == gen) mutable.value = change(mutable.value) } }
    private fun fail(gen: Long, error: Exception) {
        if (error is CancellationException) return
        val message = if (error is IOException) "网络不可用或请求超时，请检查网络后重试" else error.message ?: "更新验证失败，请重新检查"
        post(gen) { it.copy(phase = UpdatePhase.ERROR, downloaded = 0, message = message) }
    }
    private fun clearCache() { partial.delete(); ready.delete(); metadata.delete() }
}
