package io.stepes.couchverse.core.runtime

import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption

/**
 * One file per key in [directory]. Writes go to a temporary file first and are moved into place,
 * so a crash mid-write leaves the previous value rather than half of the new one.
 */
class FileStore(private val directory: File) : KeyValueStore {
    override suspend fun read(key: String): String? = file(key).takeIf { it.isFile }?.readText()

    override suspend fun write(key: String, value: String) {
        directory.mkdirs()
        val target = file(key)
        val temporary = File(directory, "${target.name}.tmp")
        temporary.writeText(value)
        Files.move(
            temporary.toPath(),
            target.toPath(),
            StandardCopyOption.ATOMIC_MOVE,
            StandardCopyOption.REPLACE_EXISTING,
        )
    }

    override suspend fun delete(key: String) {
        file(key).delete()
    }

    private fun file(key: String) = File(directory, fileName(key))

    internal companion object {
        private val SAFE = ('a'..'z') + ('A'..'Z') + ('0'..'9') + '-' + '_'

        /**
         * Keys hold `/` and other characters a file name cannot, so everything else is escaped;
         * escaping `.` too keeps names clear of `..` and of the `.tmp` files.
         */
        fun fileName(key: String): String = buildString {
            for (byte in key.encodeToByteArray()) {
                val value = byte.toInt() and 0xff
                val char = value.toChar()
                if (char in SAFE) append(char) else append("%%%02x".format(value))
            }
        }
    }
}
