import CouchverseCore
import CouchverseDesign
import SwiftUI

/// "Who's watching?" as a sheet on iPhone and iPad (D10: phones resume the last account and switch
/// from here). Picking a profile ticks, closes the sheet, and home cross-fades to the new account.
struct AccountSwitcherSheet: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.dismiss) private var dismiss
    @State private var setup: AccountSetup?
    @State private var picked = 0

    var body: some View {
        NavigationStack {
            List {
                Section {
                    ForEach(core.accounts.accounts, id: \.id) { card in
                        Button {
                            choose(card)
                        } label: {
                            AccountRow(card: card, current: card.id == core.app.activeAccount)
                        }
                        .accessibilityIdentifier("switch-\(card.username)")
                    }
                }
                Section {
                    Button(L10n.accountsAddAccount, systemImage: "person.badge.plus") { setup = .addAccount }
                }
            }
            .navigationTitle(L10n.accountsSwitch)
            .navigationBarTitleDisplayModeInline()
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(L10n.commonDone) { dismiss() }
                }
            }
        }
        .presentationDetents([.medium, .large])
        .selectionHaptic(trigger: picked)
        .accountSetup($setup)
        .onChange(of: core.signIn.signedIn) { _, account in
            if account != nil {
                dismiss()
            }
        }
    }

    private func choose(_ card: AccountCard) {
        guard card.signedIn else {
            setup = .signInAgain(card)
            return
        }
        picked += 1
        if card.id != core.app.activeAccount {
            core.send(.accountSelected(AccountRef(accountId: card.id)))
        }
        dismiss()
    }
}
