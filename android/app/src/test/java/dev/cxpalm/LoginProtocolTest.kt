package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class LoginProtocolTest {
    @Test fun onlyAccountLoginIsShippedWithoutScannerOrCameraPermission() {
        val main=java.io.File("src/main")
        assertFalse(java.io.File(main,"java/dev/cxpalm/Pairing.kt").exists())
        assertFalse(java.io.File(main,"AndroidManifest.xml").readText().contains("android.permission.CAMERA"))
        assertFalse(java.io.File("build.gradle.kts").readText().contains("zxing"))
        val bridge=java.io.File(main,"java/dev/cxpalm/BridgeClient.kt").readText()
        assertFalse(bridge.contains("/api/v1/pair"));assertFalse(bridge.contains("parsePairingLink"))
        assertFalse(java.io.File(main,"java/dev/cxpalm/PalmApp.kt").readText().contains("旧版配对"))
    }
    @Test fun trustedAddressAndBoundedLifetime() {
        assertEquals("https://host.test",loginOrigin("https://host.test/cxpalm/"))
        for(value in listOf("http://host.test","https://user:secret@host.test","https://host.test/?password=secret","https://host.test/#pair=abc","https://host.test:0","https://host.test/other")) assertTrue(runCatching {loginOrigin(value)}.isFailure)
        val value=JSONObject().put("token","a".repeat(64)).put("device",JSONObject().put("id","b".repeat(32)).put("expires",1000+30*24*3600L))
        val remembered=loginComputer(value.toString(),"https://host.test","Host",true,1000)
        assertEquals(1000+30*24*3600L,remembered.expires)
        assertFalse(remembered.toString().contains(remembered.token))
        assertTrue(runCatching {loginComputer(value.toString(),"https://host.test","",false,1000)}.isFailure)
        value.getJSONObject("device").put("expires",1000+24*3600L)
        val temporary=loginComputer(value.toString(),"https://host.test","",false,1000)
        assertEquals("[]",savedComputers(listOf(temporary)))
        val saved=org.json.JSONArray(savedComputers(listOf(temporary,remembered)))
        assertEquals(1,saved.length());assertTrue(saved.getJSONObject(0).getBoolean("remember"))
        assertEquals(remembered.token,saved.getJSONObject(0).getString("token"))
        assertFalse(saved.getJSONObject(0).has("password"))
        value.getJSONObject("device").put("expires",999)
        assertTrue(runCatching {loginComputer(value.toString(),"https://host.test","",true,1000)}.isFailure)
    }
}
