import CouchverseCore
import Foundation

/// View models for previews and snapshot tests: every screen renders from these exactly as it
/// would from the core, without a server.
public enum Fixtures {
    public static let serverId = "4f6c0a5e-6a43-4c0e-9d4b-2b8f8d0b7a11"

    public static let server = Server(
        id: serverId, url: "http://192.168.1.5:8080", name: "Home Media", version: "1.4.0",
        apiLevel: 1, accent: "#3a6ea5", insecure: true)

    public static let secureServer = Server(
        id: "9b1d7a40-1c2e-4b8f-a3d1-7f0e2c9b5a66", url: "https://media.example.com", name: "Cabin",
        version: "1.4.0", apiLevel: 1, accent: "#e50914", insecure: false)

    public static func account(
        _ username: String, _ displayName: String, server: Server = server, signedIn: Bool = true,
        rank: RankBadge? = nil, accent: AccentPalette? = nil
    ) -> AccountCard {
        AccountCard(
            id: "\(server.id)/\(username)", serverId: server.id, serverName: server.name,
            insecure: server.insecure, username: username, displayName: displayName, avatarUrl: nil,
            signedIn: signedIn, rank: rank, accent: accent)
    }

    public static let accounts = AccountsView(
        accounts: [
            account("nora", "Nora"),
            account("admin", "Štěpán"),
            account("kids", "Kids Corner"),
            account("oskar", "Oskar", server: secureServer, signedIn: false),
        ],
        active: "\(serverId)/nora")

    /// The same profiles with the ranks and banner colours this device last saw: Nora's amber
    /// banner tints the picker, Kids Corner has neither and keeps its identicon's colour.
    public static let rankedAccounts = AccountsView(
        accounts: [
            account(
                "nora", "Nora", rank: rankBadge,
                accent: AccentPalette(
                    accent: "#d97706", strong: "#a95d05", soft: "#d9770629", onAccent: "#0b0c10", ink: "#e5912c")),
            account(
                "admin", "Štěpán",
                rank: RankBadge(
                    tier: tier("marathoner", 6, 15000), next: tier("sage", 7, 25000), xp: 17200, percent: 22)),
            account("kids", "Kids Corner"),
            account(
                "oskar", "Oskar", server: secureServer, signedIn: false,
                rank: RankBadge(tier: tier("remote", 2, 500), next: tier("snack", 3, 1500), xp: 1300, percent: 80)),
        ],
        active: "\(serverId)/nora")

    public static let servers = ServersView(
        servers: [server, secureServer],
        add: AddServerView(status: .idle, address: "", added: nil, problem: nil))

    public static func addServer(_ status: LoadStatus, problem: String? = nil) -> ServersView {
        ServersView(
            servers: [],
            add: AddServerView(
                status: status, address: "media.example.com", added: nil,
                problem: problem.map { Problem(code: $0, detail: "") }))
    }

    public static func signIn(_ status: LoadStatus, problem: String? = nil) -> SignInView {
        SignInView(
            serverId: serverId, status: status, problem: problem.map { Problem(code: $0, detail: "") },
            pairing: nil, signedIn: nil)
    }

    /// A pairing that started at `now` and expires in ten minutes.
    public static func pairing(_ state: PairingState, now: UInt64 = 0) -> SignInView {
        SignInView(
            serverId: serverId, status: .idle, problem: nil,
            pairing: PairingView(
                userCode: "WDJB-MJHT", verifyUrl: "http://192.168.1.5:8080/pair?code=WDJB-MJHT",
                expiresAtMs: now + 600_000, state: state),
            signedIn: nil)
    }

    public static func session(_ status: LoadStatus, language: String = "en") -> SessionView {
        let user = SessionUser(
            username: "nora", displayName: "Nora", admin: false, avatarId: nil, bannerId: nil,
            bio: "Mostly **sci-fi**.", createdAt: "2026-01-01T00:00:00Z")
        let hasUser = status == .loaded || status == .stale
        return SessionView(
            status: status, accountId: "\(serverId)/nora", user: hasUser ? user : nil,
            features: Features(couch: true, rankings: true, downloads: true), language: language,
            accent: AccentPalette(
                accent: "#3a6ea5", strong: "#2d5681", soft: "#3a6ea529", onAccent: "#ffffff", ink: "#5b87b4"),
            problem: status == .failed || status == .stale ? Problem(code: "offline", detail: "") : nil,
            offline: status == .failed || status == .stale)
    }

    public static func devices(_ status: LoadStatus) -> DevicesView {
        let devices = [
            DeviceCard(
                id: "d1", name: "Nora's iPhone", platform: "ios", lastSeenAt: "2026-10-02T09:00:00Z", current: true),
            DeviceCard(
                id: "d2", name: "Living Room", platform: "tvos", lastSeenAt: "2026-10-01T20:15:00Z", current: false),
            DeviceCard(
                id: "d3", name: "Firefox on Linux", platform: "web", lastSeenAt: "2026-09-12T18:40:00Z", current: false),
        ]
        let shown = status == .loaded || status == .stale
        return DevicesView(
            status: status, devices: shown ? devices : [],
            problem: status == .failed ? Problem(code: "timeout", detail: "") : nil)
    }

    public static func approval(_ status: LoadStatus, outcome: ApprovalOutcome? = nil) -> PairingApprovalView {
        let loaded = status == .loaded
        return PairingApprovalView(
            status: status, code: "WDJB-MJHT", deviceName: loaded ? "Living Room" : "",
            platform: loaded ? "tvos" : "", outcome: outcome,
            problem: status == .failed ? Problem(code: "offline", detail: "") : nil)
    }

    /// The reference date device timestamps are shown relative to.
    public static let now = ISO8601DateFormatter().date(from: "2026-10-02T12:00:00Z") ?? .now
}
