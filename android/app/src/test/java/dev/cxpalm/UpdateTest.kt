package dev.cxpalm

import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
import java.io.File
import java.io.IOException
import java.net.SocketTimeoutException
import java.nio.file.Files
import java.util.concurrent.CancellationException

class UpdateTest {
    private val pkg = "io.github.u702165405.cxpalm.debug"
    private val bytes = "apk fixture, not an installable APK".toByteArray()
    private fun manifest(code: Int = 3) = UpdateManifest("0.0.2", code, pkg, "$UPDATE_ORIGIN/cxpalm/CXPalm-0.0.2-build3-debug.apk", bytes.size.toLong(), sha256(bytes), "中文说明")
    private fun rejected(block: () -> Unit) { try { block(); fail("Unsafe update accepted") } catch (_: IllegalArgumentException) { } }
    private fun transport(status: Int = 200, body: ByteArray = bytes, inspect: (Request) -> Unit = {}): UpdateTransport =
        UpdateTransport(updateHttpClient().newBuilder().addInterceptor { chain ->
            inspect(chain.request())
            Response.Builder().request(chain.request()).protocol(Protocol.HTTP_1_1).code(status).message("test")
                .header("Location", "https://outside.invalid/evil.apk").body(body.toResponseBody("application/octet-stream".toMediaType())).build()
        }.build())
    private fun temp(block: (File) -> Unit) {
        val directory = Files.createTempDirectory("cxpalm-update-test").toFile()
        try { block(File(directory, "partial.apk")) } finally { directory.listFiles()?.forEach { it.delete() }; directory.delete() }
    }

    @Test fun automaticChecksHaveSixHourMinimumAndManualPolicyIsSeparate() {
        val now = 1_000_000_000L
        assertTrue(shouldAutoCheck(now, 0))
        assertFalse(shouldAutoCheck(now, now))
        assertFalse(shouldAutoCheck(now + UPDATE_INTERVAL_MS - 1, now))
        assertTrue(shouldAutoCheck(now + UPDATE_INTERVAL_MS, now))
        assertFalse(shouldAutoCheck(now - 1, now))
    }
    @Test fun manifestRoundTripRetainsIdentityAndOptionalNotes() {
        assertEquals(manifest(), parseUpdateManifest(manifest().json(), pkg))
        val old = JSONObject(manifest().json()).remove("notes")
        assertNotNull(old)
        val raw = JSONObject(manifest().json()).apply { remove("notes"); put("apk", "CXPalm-0.0.2-build3-debug.apk") }
        assertEquals(manifest().copy(notes = ""), parseUpdateManifest(raw.toString(), pkg))
    }
    @Test fun noUpdateAndOlderManifestsDoNotOfferDownloads() {
        assertFalse(isNewerVersion(2, 2))
        assertFalse(isNewerVersion(1, 2))
        assertTrue(isNewerVersion(3, 2))
    }
    @Test fun manifestRejectsMalformedAndUnboundedFields() {
        for ((key, value) in listOf("size" to 0, "size" to MAX_APK_BYTES + 1, "size" to "12", "size" to 1.5,
            "versionCode" to 0, "versionCode" to Int.MAX_VALUE.toLong() + 1, "versionCode" to "3", "version" to 2,
            "package" to "another.app", "sha256" to "bad", "notes" to "x".repeat(4001))) {
            rejected { parseUpdateManifest(JSONObject(manifest().json()).put(key, value).toString(), pkg) }
        }
        rejected { parseUpdateManifest(" ".repeat(MAX_MANIFEST_BYTES.toInt() + 1), pkg) }
    }
    @Test fun onlyFixedHttpsApkPathsAreAccepted() {
        assertEquals("https://win.lqilt.top/cxpalm/download.json", UPDATE_MANIFEST_URL)
        assertEquals(manifest().apk, updateApkUrl("CXPalm-0.0.2-build3-debug.apk"))
        assertEquals(manifest().apk, updateApkUrl(manifest().apk))
        for (input in listOf("https://mac.lqilt.top/cxpalm/a.apk", "http://win.lqilt.top/cxpalm/a.apk", "https://win.lqilt.com/cxpalm/a.apk", "https://evil.invalid/cxpalm/a.apk",
            "https://user@win.lqilt.top/cxpalm/a.apk", "https://win.lqilt.top:443/cxpalm/a.apk", "//win.lqilt.top/cxpalm/a.apk",
            "../a.apk", "/android/a.apk", "a.apk?token=x", "a.apk#x", "%61.apk", "/cxpalm/../cxpalm/a.apk", "a.zip")) {
            rejected { updateApkUrl(input) }
        }
    }
    @Test fun packageVersionAndExactInstalledSignerAreRequired() {
        val installed = ApkIdentity(pkg, "0.0.1", 2, setOf("installed-certificate"))
        val correct = ApkIdentity(pkg, "0.0.2", 3, installed.certificates)
        verifyApkIdentity(manifest(), installed, correct)
        for (archive in listOf(correct.copy(packageName = "other"), correct.copy(version = "wrong"), correct.copy(versionCode = 2),
            correct.copy(versionCode = 1), correct.copy(versionCode = 4), correct.copy(certificates = emptySet()),
            correct.copy(certificates = setOf("other")), correct.copy(certificates = installed.certificates + "other"))) {
            rejected { verifyApkIdentity(manifest(), installed, archive) }
        }
        rejected { verifyApkIdentity(manifest(), installed.copy(certificates = emptySet()), correct) }
    }
    @Test fun successfulDownloadIsBoundedHashedAndUnauthenticated() = temp { file ->
        var progress = 0L
        transport(inspect = { request ->
            assertEquals(manifest().apk, request.url.toString())
            assertNull(request.header("Authorization")); assertNull(request.header("Origin")); assertNull(request.header("Cookie"))
            assertEquals("identity", request.header("Accept-Encoding"))
        }).download(manifest(), file, progress = { progress = it })
        assertArrayEquals(bytes, file.readBytes()); assertEquals(bytes.size.toLong(), progress)
    }
    @Test fun manifestUsesOnlyFixedPublicEndpointWithoutBearer() {
        val result = transport(body = manifest().json().toByteArray(), inspect = { request ->
            assertEquals(UPDATE_MANIFEST_URL, request.url.toString()); assertNull(request.header("Authorization")); assertNull(request.header("Origin"))
        }).manifest(pkg)
        assertEquals(manifest(), result)
    }
    @Test fun redirectAndErrorResponsesNeverBecomeDownloads() = temp { file ->
        for (code in listOf(301, 302, 307, 401, 404, 500)) {
            try { transport(status = code).download(manifest(), file); fail("Accepted HTTP $code") } catch (_: IOException) { }
            assertFalse(file.exists())
        }
        val client = updateHttpClient()
        assertFalse(client.followRedirects); assertFalse(client.followSslRedirects); assertFalse(client.retryOnConnectionFailure)
    }
    @Test fun sizeAndHashFailuresRemovePartialFiles() = temp { file ->
        for (meta in listOf(manifest().copy(size = bytes.size - 1L), manifest().copy(size = bytes.size + 1L), manifest().copy(sha256 = "0".repeat(64)))) {
            rejected { transport().download(meta, file) }; assertFalse(file.exists())
        }
    }
    @Test fun cancellationCleansUpAndRetryStartsFromZero() = temp { file ->
        file.writeText("stale partial must not be resumed")
        var n = 0
        try { transport().download(manifest(), file, active = { if (++n > 1) throw CancellationException() }); fail("Not cancelled") }
        catch (_: CancellationException) { }
        assertFalse(file.exists())
        transport().download(manifest(), file)
        assertArrayEquals(bytes, file.readBytes())
    }
    @Test fun cancelCallAndNetworkFailuresRemainRetryable() = temp { file ->
        try { transport().download(manifest(), file, onCall = { it.cancel() }); fail("Not cancelled") } catch (_: IOException) { }
        assertFalse(file.exists())
        for (failure in listOf(IOException("offline"), SocketTimeoutException("timeout"))) {
            val client = updateHttpClient().newBuilder().addInterceptor { throw failure }.build()
            try { UpdateTransport(client).download(manifest(), file); fail("Network error swallowed") } catch (_: IOException) { }
            assertFalse(file.exists())
        }
        transport().download(manifest(), file); assertArrayEquals(bytes, file.readBytes())
    }
    @Test fun unknownLengthResponsesStillEnforceActualByteLimit() = temp { file ->
        val client = updateHttpClient().newBuilder().addInterceptor { chain ->
            val source = okio.Buffer().write(bytes + bytes)
            val body = object : ResponseBody() {
                override fun contentType() = "application/octet-stream".toMediaType()
                override fun contentLength() = -1L
                override fun source() = source
            }
            Response.Builder().request(chain.request()).protocol(Protocol.HTTP_1_1).code(200).message("test").body(body).build()
        }.build()
        rejected { UpdateTransport(client).download(manifest(), file) }; assertFalse(file.exists())
        rejected { transport(body = ByteArray(MAX_MANIFEST_BYTES.toInt() + 1)).manifest(pkg) }
    }
}
