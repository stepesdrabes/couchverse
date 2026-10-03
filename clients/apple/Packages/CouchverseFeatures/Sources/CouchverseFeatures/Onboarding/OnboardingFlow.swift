import CouchverseCore
import CouchverseDesign
import SwiftUI

/// First launch (D11): welcome, add a server, sign in. It also covers a returning install whose
/// servers have no account yet, which starts at signing in.
struct OnboardingFlow: View {
    @Environment(CoreRuntime.self) private var core
    @State private var path: [SetupStep] = []
    /// Kept from the first phase seen, so adding a server pushes sign-in forward instead of
    /// replacing the welcome screen underneath it.
    @State private var startsAtWelcome: Bool

    init(startsAtWelcome: Bool) {
        _startsAtWelcome = State(initialValue: startsAtWelcome)
    }

    var body: some View {
        NavigationStack(path: $path) {
            Group {
                if startsAtWelcome {
                    WelcomeScreen { path.append(.addServer) }
                } else if let server = core.servers.servers.first {
                    SignInScreen(serverId: server.id, username: nil) { path.append(.addServer) }
                }
            }
            .setupDestinations($path)
        }
        .onServerAdded { id in
            if path.last != .signIn(serverId: id, username: nil) {
                path.append(.signIn(serverId: id, username: nil))
            }
        }
    }
}
