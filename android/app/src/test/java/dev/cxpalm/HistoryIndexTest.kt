package dev.cxpalm
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
class HistoryIndexTest {
 @Test fun lightweightIndexReusesVisibleBodiesWithoutLosingBackwardContinuity(){
  val snapshot=JSONObject("""{"turns":[{"turnId":"t9"},{"turnId":"t10"}]}""")
  val index=seedHistoryIndex(JSONObject("""{"turns":[],"questions":[],"offset":9,"total":11,"hasMore":false}"""),snapshot,true)
  assertSame(snapshot.getJSONArray("turns"),index.getJSONArray("turns"))
  val earlier=JSONObject("""{"turns":[{"turnId":"t8"}],"questions":[],"offset":8,"total":11,"hasMore":true}""")
  assertEquals(listOf("t8","t9","t10"),objects(extendHistory(index,earlier).getJSONArray("turns")).map{it.getString("turnId")})
 }
 @Test fun ordinaryHistoryPageIsNeverReplacedByCurrentConversation(){
  val page=JSONObject("""{"turns":[{"turnId":"old"}]}""");assertSame(page,seedHistoryIndex(page,JSONObject(),false));assertEquals("old",page.getJSONArray("turns").getJSONObject(0).getString("turnId"))
 }
}
