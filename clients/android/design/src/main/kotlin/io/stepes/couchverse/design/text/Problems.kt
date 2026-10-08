package io.stepes.couchverse.design.text

import androidx.annotation.StringRes
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.design.R

/** The message for a problem the core reported; its code is stable, its detail never shown. */
@Composable
fun problemMessage(problem: Problem?): String = stringResource(problemText(problem?.code))

@StringRes
fun problemText(code: String?): Int = when (code) {
    "offline", "network" -> R.string.problem_offline
    "timeout" -> R.string.problem_timeout
    "tls" -> R.string.problem_tls
    "invalid_address" -> R.string.problem_invalid_address
    "not_a_server" -> R.string.problem_not_a_server
    "server_outdated" -> R.string.problem_server_outdated
    "invalid_credentials" -> R.string.problem_invalid_credentials
    "rate_limited", "slow_down" -> R.string.problem_rate_limited
    "invalid_code" -> R.string.problem_invalid_code
    "invalid_password" -> R.string.problem_invalid_password
    "unauthorized" -> R.string.problem_unauthorized
    "forbidden" -> R.string.problem_forbidden
    "conflict" -> R.string.problem_conflict
    "not_found" -> R.string.error_not_found
    "not_host" -> R.string.problem_not_host
    "session_full" -> R.string.problem_session_full
    "already_hosting" -> R.string.problem_already_hosting
    else -> R.string.problem_generic
}
