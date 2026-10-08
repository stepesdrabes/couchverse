import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The first screen of a fresh install: what Couchverse is, the ways to reach a server and, for a
/// guest without an account, joining someone's couch.
struct WelcomeScreen: View {
    let onAddServer: () -> Void

    @Environment(CoreRuntime.self) private var core
    @Environment(\.accent) private var accent
    @Environment(\.showCouchJoin) private var showCouchJoin
    @State private var scanning = false
    /// A scanned connect link is being redeemed; its outcome shows here until sign-in takes over.
    @State private var connecting = false

    var body: some View {
        ZStack {
            GlowBackdrop(tint: accent.color)
            ScrollView {
                VStack(spacing: Idiom.isTV ? Tokens.Spacing.xxxl : Tokens.Spacing.xl) {
                    Image(systemName: "sofa.fill")
                        .font(.system(size: Idiom.isTV ? 120 : 64, weight: .semibold))
                        .foregroundStyle(accent.color)
                        .shadow(color: accent.color.opacity(0.6), radius: 30)
                        .accessibilityHidden(true)
                    VStack(spacing: Tokens.Spacing.md) {
                        Text(L10n.onboardingWelcomeTitle)
                            .typeRole(Tokens.TypeRamp.hero)
                            .foregroundStyle(Tokens.Palette.text)
                            .accessibilityAddTraits(.isHeader)
                        Text(L10n.onboardingWelcomeMessage)
                            .typeRole(Tokens.TypeRamp.body)
                            .foregroundStyle(Tokens.Palette.muted)
                    }
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)

                    if connecting, core.signIn.status == .failed, let problem = core.signIn.problem {
                        ProblemBanner(problem)
                    }

                    actions
                }
                .readableWidth(Idiom.isTV ? 900 : 520)
                .padding(Tokens.Spacing.xl)
                .containerRelativeFrame(.vertical, alignment: .center) { length, _ in length }
            }
            .scrollBounceBehavior(.basedOnSize)
        }
        .toolbarVisibility(.hidden, for: .navigationBar)
        #if os(iOS)
            .sheet(isPresented: $scanning) {
                QRScannerSheet(expecting: .connect) { url in
                    scanning = false
                    connecting = true
                    core.send(.linkOpened(Link(url: url)))
                }
            }
        #endif
    }

    @ViewBuilder private var actions: some View {
        let busy = connecting && core.signIn.status == .loading
        VStack(spacing: Tokens.Spacing.md) {
            Button(action: onAddServer) {
                ActionLabel(L10n.onboardingAddServer, systemImage: "server.rack")
            }
            .primaryAction()
            .disabled(busy)
            #if os(iOS)
                Button {
                    scanning = true
                } label: {
                    ActionLabel(
                        busy ? L10n.onboardingConnecting : L10n.onboardingScanQr, systemImage: "qrcode.viewfinder",
                        busy: busy)
                }
                .secondaryAction()
                .disabled(busy)
                Text(L10n.onboardingScanHint)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.muted)
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)
            #endif
            Button {
                showCouchJoin()
            } label: {
                ActionLabel(L10n.couchJoinTitle, systemImage: "sofa")
            }
            .secondaryAction()
            .disabled(busy)
            .accessibilityIdentifier("welcome-join-couch")
        }
        .frame(maxWidth: Idiom.isTV ? 600 : .infinity)
    }
}
