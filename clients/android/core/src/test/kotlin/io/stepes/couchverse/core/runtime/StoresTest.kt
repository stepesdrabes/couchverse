package io.stepes.couchverse.core.runtime

import kotlinx.coroutines.test.runTest
import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class StoresTest {
    private val directory: File = Files.createTempDirectory("store").toFile()

    @Test
    fun `values round-trip under keys no file name could hold`() = runTest {
        val store = FileStore(directory)
        val key = "token.4f6c0a5e-6a43/2"
        assertNull(store.read(key))

        store.write(key, "tok-1")
        store.write(key, "tok-2")
        store.write("warm.a/1.home", """{"featured":[]}""")
        assertEquals("tok-2", store.read(key))
        assertEquals("""{"featured":[]}""", store.read("warm.a/1.home"))

        store.delete(key)
        assertNull(store.read(key))
        assertTrue(directory.listFiles().orEmpty().none { it.name.endsWith(".tmp") })
    }

    @Test
    fun `file names stay inside the directory`() {
        val name = FileStore.fileName("../../etc/passwd")
        assertFalse(name.contains('/'))
        assertFalse(name.contains(".."))
    }

    @Test
    fun `secrets are sealed on disk and unreadable ones read as missing`() = runTest {
        val files = FileStore(directory)
        val store = EncryptedStore(files, ReversingBox)

        store.write("token.a", "secret-token")
        assertEquals("secret-token", store.read("token.a"))
        assertFalse(files.read("token.a").orEmpty().contains("secret"))

        // a key lost to a restore on another device
        files.write("token.b", "bm90IHNlYWxlZA==")
        assertNull(store.read("token.b"))
        files.write("token.c", "not base64 at all!")
        assertNull(store.read("token.c"))
    }

    /** Stands in for the Keystore: "seals" by reversing behind a marker byte. */
    private object ReversingBox : SecretBox {
        private const val MARK: Byte = 0x7f

        override fun seal(plain: ByteArray) = byteArrayOf(MARK) + plain.reversedArray()

        override fun open(sealed: ByteArray) =
            if (sealed.firstOrNull() == MARK) sealed.drop(1).toByteArray().reversedArray() else null
    }
}
