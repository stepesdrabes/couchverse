import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Signing in to one server: a password, or another device approving a pairing code. A TV leads
/// with the code (typing on a remote is slow) and shows both side by side; a phone leads with the
/// password and shows the code on request.
struct SignInScreen: View {
    let serverId: String
    let username: String?
    var onChangeServer: (() -> Void)?

    @Environment(CoreRuntime.self) private var core
    @State private var user = ""
    @State private var password = ""
    @State private var submitted = false
    @State private var pairingRequested = false
    @FocusState private var field: Field?

    private enum Field { case username, password }

    private var server: Server? { core.servers.servers.first { $0.id == serverId } }
    /// The sign-in view describes this server's attempt (or a connect link still on its way).
    private var ours: Bool { core.signIn.serverId == serverId || core.signIn.serverId == nil }
    private var pairing: PairingView? { ours ? core.signIn.pairing : nil }
    private var loading: Bool { ours && core.signIn.status == .loading }
    /// A TV shows the code and the password form at once, so it tells their attempts apart; a
    /// phone shows one at a time (a connect link's redemption counts as the password's).
    private var passwordAttempt: Bool { Idiom.isTV ? submitted : !showsPairing }
    /// A phone swaps the password form for the code while a pairing runs.
    private var showsPairing: Bool { Idiom.isTV || pairingRequested || pairing != nil }
    private var signingIn: Bool { loading && pairing == nil && passwordAttempt }
    private var problem: Problem? { ours && core.signIn.status == .failed ? core.signIn.problem : nil }

    var body: some View {
        ScrollView {
            Group {
                if Idiom.isTV {
                    // two columns from the top, so the remote moves straight across between them
                    HStack(alignment: .top, spacing: 64) {
                        VStack(alignment: .leading, spacing: Tokens.Spacing.xxl) {
                            header
                            pairingPanel
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .tvFocusSection()
                        Rectangle()
                            .fill(Tokens.Palette.edgeLine)
                            .frame(width: 1)
                        VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                            Text(L10n.accountsUsePassword)
                                .typeRole(Tokens.TypeRamp.section)
                                .foregroundStyle(Tokens.Palette.text)
                                .accessibilityAddTraits(.isHeader)
                            passwordForm
                        }
                        .frame(width: 540)
                        .tvFocusSection()
                    }
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.horizontal, 80)
                    .padding(.vertical, 60)
                } else {
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                        header
                        if showsPairing {
                            pairingPanel
                            Button(L10n.accountsUsePassword, systemImage: "key.fill") {
                                core.send(.pairingCancelled)
                                pairingRequested = false
                            }
                            .secondaryAction()
                        } else {
                            passwordForm
                            orDivider
                            Button {
                                pairingRequested = true
                                startPairing()
                            } label: {
                                ActionLabel(L10n.accountsSignInWithDevice, systemImage: "qrcode")
                            }
                            .secondaryAction()
                        }
                    }
                    .readableWidth(520)
                    .padding(Tokens.Spacing.xl)
                }
            }
            .motion(Tokens.Motion.smooth, value: showsPairing)
            .motion(Tokens.Motion.smooth, value: core.signIn)
        }
        .scrollBounceBehavior(.basedOnSize)
        // the field a remote user needs first; the code next to it needs no focus at all
        .defaultFocus($field, Idiom.isTV ? .username : nil)
        .background { GlowBackdrop(tint: Color(hex: server?.accent ?? "") ?? Tokens.Palette.accent, intensity: 0.7) }
        .navigationBarTitleDisplayModeInline()
        .onAppear {
            if let username, user.isEmpty {
                user = username
            }
            if Idiom.isTV && pairing == nil {
                startPairing()
            }
        }
        .onDisappear {
            if pairing != nil {
                core.send(.pairingCancelled)
            }
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            Text(L10n.accountsSignInTitle(server: server?.name ?? ""))
                .typeRole(Tokens.TypeRamp.title)
                .foregroundStyle(Tokens.Palette.text)
                .accessibilityAddTraits(.isHeader)
            // the badge under the address where the two do not fit on a line
            ViewThatFits(in: .horizontal) {
                HStack(spacing: Tokens.Spacing.sm) { address }
                VStack(alignment: .leading, spacing: Tokens.Spacing.xs) { address }
            }
            if let onChangeServer {
                Button(L10n.accountsChangeServer, action: onChangeServer)
                    .buttonStyle(.borderless)
                    .typeRole(Tokens.TypeRamp.caption)
            }
        }
    }

    @ViewBuilder private var address: some View {
        Text(server.map { PairingPanel.displayed($0.url) } ?? "")
            .typeRole(Tokens.TypeRamp.caption)
            .foregroundStyle(Tokens.Palette.mutedText)
        if server?.insecure == true {
            InsecureBadge()
        }
    }

    private var pairingPanel: some View {
        PairingPanel(
            pairing: pairing, loading: loading, problem: passwordAttempt ? nil : problem, onNewCode: startPairing)
    }

    @ViewBuilder private var passwordForm: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
            FormField(L10n.loginUsername, text: $user, kind: .username)
                .focused($field, equals: .username)
                .submitLabel(.next)
                .onSubmit { field = .password }
            FormField(L10n.loginPassword, text: $password, kind: .password)
                .focused($field, equals: .password)
                .submitLabel(.go)
                .onSubmit(signIn)
            if passwordAttempt, let problem {
                ProblemBanner(problem)
            }
            Button(action: signIn) {
                ActionLabel(signingIn ? L10n.accountsSigningIn : L10n.loginSubmit, busy: signingIn)
            }
            .primaryAction()
            .disabled(user.isEmpty || password.isEmpty || signingIn)
        }
        .disabled(signingIn)
    }

    private var orDivider: some View {
        HStack(spacing: Tokens.Spacing.md) {
            Rectangle().fill(Tokens.Palette.edgeLine).frame(height: 1)
            Text(L10n.accountsOr)
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.faintText)
            Rectangle().fill(Tokens.Palette.edgeLine).frame(height: 1)
        }
        .accessibilityHidden(true)
    }

    private func signIn() {
        guard !user.isEmpty, !password.isEmpty, !signingIn else { return }
        submitted = true
        field = nil
        core.send(
            .passwordSignInSubmitted(PasswordSignIn(serverId: serverId, username: user, password: password)))
    }

    private func startPairing() {
        submitted = false
        core.send(.pairingStarted(ServerRef(serverId: serverId)))
    }
}
