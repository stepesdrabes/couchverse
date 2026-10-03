import Foundation

// Placeholders a runtime starts from; a live runtime replaces them with the core's own views
// before anything is shown.

extension ServersView {
    public static let empty = ServersView(
        servers: [], add: AddServerView(status: .idle, address: "", added: nil, problem: nil))
}

extension AccountsView {
    public static let empty = AccountsView(accounts: [], active: nil)
}

extension SignInView {
    public static let idle = SignInView(
        serverId: nil, status: .idle, problem: nil, pairing: nil, signedIn: nil)
}

extension DevicesView {
    public static let idle = DevicesView(status: .idle, devices: [], problem: nil)
}

extension PairingApprovalView {
    public static let idle = PairingApprovalView(
        status: .idle, code: "", deviceName: "", platform: "", outcome: nil, problem: nil)
}

extension AccentPalette {
    /// The design tokens' default red, as the core derives it.
    public static let fallback = AccentPalette(
        accent: "#e50914", strong: "#b30710", soft: "#e5091429", onAccent: "#ffffff", ink: "#ec474f")
}

extension SessionView {
    public static func signedOut(language: String) -> SessionView {
        SessionView(
            status: .idle, accountId: nil, user: nil,
            features: Features(couch: true, rankings: true, downloads: true), language: language,
            accent: .fallback, problem: nil, offline: false)
    }
}
