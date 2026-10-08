import CouchverseCore

extension Problem {
    /// The message for the core's stable code (6.3: the core never returns user-facing text).
    public var message: String {
        switch code {
        case "invalid_address": L10n.problemInvalidAddress
        case "not_a_server": L10n.problemNotAServer
        case "server_outdated": L10n.problemServerOutdated
        case "offline", "network": L10n.problemOffline
        case "timeout": L10n.problemTimeout
        case "tls": L10n.problemTls
        case "invalid_credentials": L10n.problemInvalidCredentials
        case "invalid_password": L10n.problemInvalidPassword
        case "rate_limited", "slow_down": L10n.problemRateLimited
        case "invalid_code": L10n.problemInvalidCode
        case "unauthorized": L10n.problemUnauthorized
        case "no_session", "session_ended": L10n.couchJoinFailed
        case "not_host": L10n.problemNotHost
        case "session_full": L10n.problemSessionFull
        default: L10n.problemGeneric
        }
    }
}
