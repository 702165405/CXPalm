package dev.cxpalm

import org.junit.Assert.*
import org.junit.Assume.assumeTrue
import org.junit.Test
import java.nio.file.Files

class UpdateReleaseTest {
    @Test fun publishedFeedAndDownloadVerifyWithoutLogin() {
        assumeTrue("Opt-in public release check; no phone or Agent credentials", System.getenv("CXPALM_LIVE_UPDATE_CHECK") == "1")
        val manifest = UpdateTransport().manifest("io.github.u702165405.cxpalm.debug")
        assertEquals(requireNotNull(System.getenv("CXPALM_EXPECTED_VERSION_CODE")).toInt(), manifest.versionCode)
        val directory = Files.createTempDirectory("cxpalm-public-release").toFile()
        val apk = java.io.File(directory, "release.apk")
        try {
            UpdateTransport().download(manifest, apk)
            verifyUpdateFile(apk, manifest)
            assertEquals(manifest.size, apk.length())
        } finally { apk.delete(); directory.delete() }
    }
}
