import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The picker over the core's accounts: first thing on a TV (D10), and on any device when the
/// last account cannot resume. Choosing a signed-out profile signs it in again instead.
struct WhosWatchingScreen: View {
    /// Set when the picker covers the running app to switch profiles; it closes once a profile
    /// is chosen or the user backs out.
    var onClose: (() -> Void)?

    @Environment(CoreRuntime.self) private var core
    @Environment(ProfileChoreography.self) private var choreography
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var setup: AccountSetup?
    @State private var signingOut: AccountCard?
    @State private var selection = 0

    var body: some View {
        ProfilePicker(
            accounts: core.accounts.accounts,
            active: core.accounts.active,
            onSelect: choose,
            onAdd: { setup = .addAccount },
            onSignOut: { signingOut = $0 }
        )
        .onBack(perform: onClose)
        .selectionHaptic(trigger: selection)
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
        .accountSetup($setup)
    }

    private func choose(_ card: AccountCard, from frame: CGRect) {
        guard card.signedIn else {
            setup = .signInAgain(card)
            return
        }
        selection += 1
        let accounts = core.accounts.accounts
        if Idiom.isTV && !reduceMotion && card.id != core.app.activeAccount {
            // the focused tile is drawn lifted by 12%; layout frames do not include that
            let lifted = frame.insetBy(dx: -frame.width * 0.06, dy: -frame.height * 0.06)
            Task { await choreography.run(card: card, accounts: accounts, from: lifted) }
        }
        core.send(.accountSelected(AccountRef(accountId: card.id)))
        onClose?()
    }
}
