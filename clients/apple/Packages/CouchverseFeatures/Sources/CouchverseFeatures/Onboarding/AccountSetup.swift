import CouchverseCore
import CouchverseDesign
import SwiftUI

/// A screen in the add-server and sign-in flows.
enum SetupStep: Hashable {
    case addServer
    case signIn(serverId: String, username: String?)
}

/// Adding an account while the app is running: from the "+" profile, Settings, or a signed-out
/// account that needs to sign in again.
enum AccountSetup: Identifiable, Hashable {
    case addAccount
    case addServer
    case signInAgain(AccountCard)

    var id: String {
        switch self {
        case .addAccount: "add-account"
        case .addServer: "add-server"
        case .signInAgain(let card): "sign-in-\(card.id)"
        }
    }
}

extension View {
    func accountSetup(_ setup: Binding<AccountSetup?>) -> some View {
        modal(item: setup) { AccountSetupFlow(start: $0) }
    }

    /// Runs `action` with a server's id once an address the user (or a scanned link) submitted
    /// checked out.
    func onServerAdded(_ action: @escaping (String) -> Void) -> some View {
        modifier(ServerAddedObserver(action: action))
    }

    func setupDestinations(_ path: Binding<[SetupStep]>) -> some View {
        navigationDestination(for: SetupStep.self) { step in
            switch step {
            case .addServer:
                AddServerScreen()
            case .signIn(let serverId, let username):
                SignInScreen(serverId: serverId, username: username) {
                    path.wrappedValue.append(.addServer)
                }
            }
        }
    }
}

private struct ServerAddedObserver: ViewModifier {
    let action: (String) -> Void
    @Environment(CoreRuntime.self) private var core

    func body(content: Content) -> some View {
        content.onChange(of: core.servers.add) { old, new in
            if old.status == .loading, new.status == .loaded, let id = new.added {
                action(id)
            }
        }
    }
}

/// The add-account flow, closed as soon as an account signs in.
struct AccountSetupFlow: View {
    let start: AccountSetup
    @Environment(CoreRuntime.self) private var core
    @Environment(\.dismiss) private var dismiss
    @State private var path: [SetupStep] = []

    var body: some View {
        NavigationStack(path: $path) {
            root
                .toolbar {
                    if !Idiom.isTV {
                        ToolbarItem(placement: .cancellationAction) {
                            Button(L10n.commonCancel) { dismiss() }
                        }
                    }
                }
                .setupDestinations($path)
        }
        .onServerAdded { path.append(.signIn(serverId: $0, username: nil)) }
        .onChange(of: core.signIn.signedIn) { _, account in
            if account != nil {
                dismiss()
            }
        }
    }

    @ViewBuilder private var root: some View {
        switch start {
        case .addServer:
            AddServerScreen()
        case .signInAgain(let card):
            SignInScreen(serverId: card.serverId, username: card.username) { path.append(.addServer) }
        case .addAccount:
            if core.servers.servers.count == 1, let server = core.servers.servers.first {
                SignInScreen(serverId: server.id, username: nil) { path.append(.addServer) }
            } else {
                ServerChooser(
                    onPick: { path.append(.signIn(serverId: $0, username: nil)) },
                    onAdd: { path.append(.addServer) })
            }
        }
    }
}

/// Which server to sign in to, when there is more than one.
struct ServerChooser: View {
    let onPick: (String) -> Void
    let onAdd: () -> Void
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        List {
            Section {
                ForEach(core.servers.servers, id: \.id) { server in
                    Button {
                        onPick(server.id)
                    } label: {
                        ServerLabel(name: server.name, url: server.url, insecure: server.insecure)
                    }
                }
            }
            Section {
                Button(L10n.onboardingAddServer, systemImage: "plus") { onAdd() }
            }
        }
        .navigationTitle(L10n.accountsChooseServer)
    }
}
