package io.stepes.couchverse.accounts

import android.net.Uri

/**
 * Whether scanned text is one of the links the core opens (`couchverse://connect`,
 * `couchverse://pair`, or a pairing page); the scanner only uses it to tell the user that some
 * other QR code is not a Couchverse one. The core parses the link itself.
 */
fun isCouchverseLink(text: String): Boolean {
    val uri = runCatching { Uri.parse(text.trim()) }.getOrNull() ?: return false
    return when (uri.scheme?.lowercase()) {
        "couchverse" -> uri.host == "connect" || uri.host == "pair" || uri.host == "couch"
        "http", "https" -> uri.path?.trimEnd('/') == "/pair" && uri.getQueryParameter("code") != null ||
            uri.path?.trimEnd('/')?.matches(Regex("/couch/\\d{6}")) == true
        else -> false
    }
}

/** Whether a link signs this device in (as opposed to approving another device). */
fun isConnectLink(text: String): Boolean = text.trim().startsWith("couchverse://connect", ignoreCase = true)
