package io.stepes.couchverse.core.runtime

import java.util.Base64

/** Authenticated encryption under a key the app never sees (the Android Keystore). */
interface SecretBox {
    fun seal(plain: ByteArray): ByteArray

    /** The plain bytes, or `null` when [sealed] cannot be opened (tampered, or the key is gone). */
    fun open(sealed: ByteArray): ByteArray?
}

/**
 * The `secureStore` effect: values are sealed by [box] before [files] keeps them. A value that no
 * longer opens (the Keystore key was lost, e.g. after a restore onto another device) reads as
 * missing, so the core asks the user to sign in again instead of failing.
 */
class EncryptedStore(
    private val files: KeyValueStore,
    private val box: SecretBox,
) : KeyValueStore {
    override suspend fun read(key: String): String? {
        val stored = files.read(key) ?: return null
        val sealed = runCatching { Base64.getDecoder().decode(stored) }.getOrNull() ?: return null
        return box.open(sealed)?.decodeToString()
    }

    override suspend fun write(key: String, value: String) {
        files.write(key, Base64.getEncoder().encodeToString(box.seal(value.encodeToByteArray())))
    }

    override suspend fun delete(key: String) = files.delete(key)
}
