package io.stepes.couchverse.ranks

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Test
import java.io.File
import kotlin.test.assertEquals

/** Every code the server can send has words: the core passes the API's codes on unchanged. */
class WordsTest {
    private fun contract(path: String): JsonObject = Json.parseToJsonElement(File("../../../contract/$path").readText()).jsonObject

    /** The values the API allows for [schema]'s [property]. */
    private fun codes(schema: String, property: String): List<String> =
        contract("openapi.json").getValue("components").jsonObject.getValue("schemas").jsonObject
            .getValue(schema).jsonObject.getValue("properties").jsonObject.getValue(property).jsonObject
            .getValue("enum").jsonArray.map { it.jsonPrimitive.content }

    @Test
    fun `every xp source and tier the api names has words`() {
        assertEquals(emptyList(), codes("XPSource", "key").filterNot(XpSources::containsKey))
        assertEquals(emptyList(), codes("RankTier", "code").filterNot(TierNames::containsKey))
    }

    @Test
    fun `every achievement the catalogs name is listed`() {
        val named = contract("i18n/en.json").keys.mapNotNull { Regex("^achievement_(.+)_name$").find(it)?.groupValues?.get(1) }
        assertEquals(emptyList(), named.filterNot(AchievementNames::containsKey))
        assertEquals(named.sorted(), AchievementDescriptions.keys.sorted())
    }
}
