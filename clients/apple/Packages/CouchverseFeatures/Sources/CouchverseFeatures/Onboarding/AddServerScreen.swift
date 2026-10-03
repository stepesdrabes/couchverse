import CouchverseCore
import CouchverseDesign
import SwiftUI

/// An address to check against the server's identity (https first, then http). The flow moves on
/// to signing in once the core accepts it.
struct AddServerScreen: View {
    @Environment(CoreRuntime.self) private var core
    @State private var address = ""
    @FocusState private var fieldFocused: Bool

    private var add: AddServerView { core.servers.add }
    private var checking: Bool { add.status == .loading }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                    Text(L10n.serversAddTitle)
                        .typeRole(Tokens.TypeRamp.title)
                        .foregroundStyle(Tokens.Palette.text)
                        .accessibilityAddTraits(.isHeader)
                    Text(L10n.serversAddressHint)
                        .typeRole(Tokens.TypeRamp.body)
                        .foregroundStyle(Tokens.Palette.muted)
                }
                .fixedSize(horizontal: false, vertical: true)

                FormField(
                    L10n.serversAddressLabel, text: $address, prompt: L10n.serversAddressPlaceholder,
                    kind: .address
                )
                .focused($fieldFocused)
                .submitLabel(.go)
                .onSubmit(connect)
                .disabled(checking)

                if add.status == .failed, let problem = add.problem {
                    ProblemBanner(problem)
                        .transition(.opacity.combined(with: .move(edge: .top)))
                }

                Button(action: connect) {
                    ActionLabel(checking ? L10n.serversChecking : L10n.serversConnect, busy: checking)
                }
                .primaryAction()
                .disabled(address.trimmingCharacters(in: .whitespaces).isEmpty || checking)
            }
            .readableWidth(Idiom.isTV ? 900 : 520)
            .padding(Tokens.Spacing.xl)
            .animation(Tokens.Motion.smooth, value: add.status)
        }
        .scrollBounceBehavior(.basedOnSize)
        .background { GlowBackdrop(tint: Tokens.Palette.glowViolet, intensity: 0.6) }
        .navigationBarTitleDisplayModeInline()
        .onAppear {
            // back on this screen after a failed attempt: show what was tried
            if address.isEmpty, add.status == .failed {
                address = add.address
            }
            fieldFocused = !Idiom.isTV
        }
    }

    private func connect() {
        guard !address.trimmingCharacters(in: .whitespaces).isEmpty, !checking else { return }
        core.send(.serverAddressSubmitted(ServerAddress(address: address)))
    }
}

extension View {
    func navigationBarTitleDisplayModeInline() -> some View {
        #if os(iOS)
            navigationBarTitleDisplayMode(.inline)
        #else
            self
        #endif
    }
}
