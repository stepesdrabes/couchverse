import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Accounts, servers, display language, devices, approving another device and joining a couch
/// (plan 10.4).
struct SettingsScreen: View {
    @Environment(CoreRuntime.self) private var core
    @State private var setup: AccountSetup?
    @State private var approving = false
    @State private var signingOut: AccountCard?
    @State private var removing: Server?

    var body: some View {
        List {
            ProfileSettingsSection()
            accounts
            servers
            language
            Section {
                NavigationLink {
                    DevicesScreen()
                } label: {
                    Label(L10n.devicesTitle, systemImage: "laptopcomputer.and.iphone")
                }
                DownloadsLink()
                Button {
                    approving = true
                } label: {
                    Label(L10n.pairingApproveTitle, systemImage: "qrcode.viewfinder")
                }
                .accessibilityIdentifier("approve-device")
            }
            CouchSettingsSection()
            about
        }
        .navigationTitle(L10n.navSettings)
        .accountSetup($setup)
        .modal(isPresented: $approving) { ApproveDeviceScreen() }
        .confirmationDialog(
            L10n.accountsSignOutConfirmTitle(name: signingOut?.displayName ?? ""),
            isPresented: Binding(get: { signingOut != nil }, set: { if !$0 { signingOut = nil } }),
            titleVisibility: .visible
        ) {
            Button(L10n.navSignOut, role: .destructive) {
                if let card = signingOut {
                    core.send(.signOutRequested(AccountRef(accountId: card.id)))
                }
            }
        } message: {
            Text(L10n.accountsSignOutConfirmMessage)
        }
        .confirmationDialog(
            L10n.serversRemoveConfirmTitle(name: removing?.name ?? ""),
            isPresented: Binding(get: { removing != nil }, set: { if !$0 { removing = nil } }),
            titleVisibility: .visible
        ) {
            Button(L10n.serversRemove, role: .destructive) {
                if let server = removing {
                    core.send(.serverRemoved(ServerRef(serverId: server.id)))
                }
            }
        } message: {
            Text(L10n.serversRemoveConfirmMessage)
        }
    }

    private var accounts: some View {
        Section(L10n.accountsTitle) {
            ForEach(core.accounts.accounts, id: \.id) { card in
                Button {
                    if !card.signedIn {
                        setup = .signInAgain(card)
                    } else if card.id != core.app.activeAccount {
                        core.send(.accountSelected(AccountRef(accountId: card.id)))
                    }
                } label: {
                    AccountRow(card: card, current: card.id == core.app.activeAccount)
                }
                .contextMenu {
                    Button(L10n.navSignOut, systemImage: "rectangle.portrait.and.arrow.right", role: .destructive) {
                        signingOut = card
                    }
                }
                .swipeToRemove(L10n.navSignOut) { signingOut = card }
            }
            Button(L10n.accountsAddAccount, systemImage: "person.badge.plus") { setup = .addAccount }
        }
    }

    private var servers: some View {
        Section(L10n.serversTitle) {
            ForEach(core.servers.servers, id: \.id) { server in
                VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                    ServerLabel(name: server.name, url: server.url, insecure: server.insecure)
                    Text(L10n.serversVersion(version: server.version))
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.faint)
                }
                .focusable(Idiom.isTV)
                .contextMenu {
                    Button(L10n.serversRemove, systemImage: "trash", role: .destructive) { removing = server }
                }
                .swipeToRemove(L10n.serversRemove) { removing = server }
            }
            Button(L10n.onboardingAddServer, systemImage: "plus") { setup = .addServer }
        }
    }

    private var language: some View {
        Section(L10n.languageLabel) {
            ForEach(L10n.languages, id: \.self) { code in
                Button {
                    core.send(.displayLanguageChanged(LanguageChoice(code: code)))
                } label: {
                    HStack {
                        Text(code == "cs" ? L10n.langCzech : L10n.langEnglish)
                            .foregroundStyle(Tokens.Palette.text)
                        Spacer()
                        if core.session.language == code {
                            Image(systemName: "checkmark").foregroundStyle(.tint)
                        }
                    }
                }
                .accessibilityAddTraits(core.session.language == code ? .isSelected : [])
            }
        }
    }

    private var about: some View {
        Section(L10n.aboutTitle) {
            LabeledContent(L10n.aboutAppVersion, value: Self.appVersion)
            if let server = activeServer {
                LabeledContent(L10n.aboutServerVersion, value: "\(server.name) \(server.version)")
            }
        }
        .focusable(Idiom.isTV)
    }

    private var activeServer: Server? {
        let serverId = core.accounts.accounts.first { $0.id == core.app.activeAccount }?.serverId
        return core.servers.servers.first { $0.id == serverId }
    }

    private static var appVersion: String {
        let info = Bundle.main.infoDictionary
        let version = info?["CFBundleShortVersionString"] as? String ?? "0"
        let build = info?["CFBundleVersion"] as? String ?? "0"
        return "\(version) (\(build))"
    }
}

/// An account in a list: avatar, name, server, and whether it is the one in use or signed out.
struct AccountRow: View {
    let card: AccountCard
    let current: Bool

    var body: some View {
        HStack(spacing: Tokens.Spacing.md) {
            AvatarView(url: card.avatarUrl, seed: card.username, name: card.displayName)
                .frame(width: Idiom.isTV ? 64 : 40, height: Idiom.isTV ? 64 : 40)
                .saturation(card.signedIn ? 1 : 0)
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                Text(card.displayName)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.text)
                Text(card.signedIn ? card.serverName : "\(card.serverName) \u{00B7} \(L10n.accountsSignedOut)")
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.muted)
            }
            Spacer()
            if current {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(.tint)
                    .accessibilityLabel(L10n.accountsCurrent)
            } else if !card.signedIn {
                Text(L10n.accountsSignInAgain)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(.tint)
            }
        }
        .accessibilityElement(children: .combine)
    }
}

extension View {
    /// Swipe to sign out or remove on touch devices; a TV uses the context menu instead.
    @ViewBuilder func swipeToRemove(_ title: String, enabled: Bool = true, action: @escaping () -> Void) -> some View {
        #if os(iOS)
            swipeActions {
                if enabled {
                    Button(title, systemImage: "trash", role: .destructive, action: action)
                }
            }
        #else
            self
        #endif
    }
}
