package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class DirectSettingsTest {
    @Test fun currentValuesAreSelectedWithoutInheritanceAndCustomPermissionsStayUnknown() {
        val current=sessionControlValues(JSONObject("""{"latestModel":"actual","latestReasoningEffort":"high","latestThreadSettings":{"activePermissionProfile":{"id":":danger-full-access"}},"latestCollaborationMode":{"mode":"plan"}}"""))
        assertEquals(mapOf("model" to "actual","effort" to "high","permission" to "full","mode" to "plan"),current)
        assertEquals("",sessionControlValues(JSONObject("""{"latestThreadSettings":{"activePermissionProfile":{"id":"custom"}}}"""))["permission"])
        assertEquals("default",sessionControlValues(null)["effort"])
        assertEquals("workspace",sessionControlValues(JSONObject("""{"currentPermissions":{"sandboxPolicy":{"type":"workspace-write"}}}"""))["permission"])
    }
    @Test fun unchangedFullAccessNeedsNoWriteAndChangingModelCannotWritePermissionsOrMode() {
        val base=mapOf("model" to "actual","effort" to "high","permission" to "full","mode" to "plan")
        assertTrue(changedControlSettings(base,base,"permission").isEmpty())
        assertEquals(mapOf("model" to "new"),changedControlSettings(base,base+mapOf("model" to "new","permission" to "workspace","mode" to "default"),"model"))
        assertEquals(mapOf("permission" to "read-only"),changedControlSettings(base,base+("permission" to "read-only"),"permission"))
        assertEquals(mapOf("effort" to "default"),changedControlSettings(base,base+("effort" to "default"),"model"))
    }
}
