import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The session at a glance: for the host the code and a QR code of the join page to pass around,
/// then who is on the couch, and the way out (ending it for everyone, for the host). Large on TV,
/// where friends read the code off the screen from across the room. `onLeft` runs after this
/// device ends or leaves the session.
struct CouchPanel: View {
    let view: CouchView
    var onLeft: () -> Void = {}

    @Environment(CoreRuntime.self) private var core
    @State private var ending = false
    @ScaledMetric(relativeTo: .largeTitle) private var codeSize: CGFloat = Idiom.isTV ? 96 : 40
    @ScaledMetric(relativeTo: .body) private var qrSize: CGFloat = Idiom.isTV ? 420 : 180

    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.xxl : Tokens.Spacing.xl) {
            if view.isLive {
                live
            } else if view.status == .connecting {
                HStack(spacing: Tokens.Spacing.md) {
                    ProgressView()
                    Text(L10n.couchStarting)
                }
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.muted)
                .accessibilityElement(children: .combine)
            } else if view.status == .ended {
                CatalogMessage(systemImage: "sofa", title: CouchLabels.status(view) ?? "", message: nil)
            } else if view.problem != nil {
                CatalogMessage(
                    systemImage: "exclamationmark.triangle", title: L10n.couchStartFailed, message: nil)
                startButton
            } else {
                Text(L10n.couchStartSessionHint)
                    .typeRole(Tokens.TypeRamp.body)
                    .foregroundStyle(Tokens.Palette.muted)
                    .fixedSize(horizontal: false, vertical: true)
                startButton
            }
        }
        .animation(Tokens.Motion.smooth, value: view)
    }

    @ViewBuilder private var live: some View {
        Text(L10n.couchOnCouchCount(count: String(view.members.count)))
            .typeRole(Tokens.TypeRamp.title)
            .foregroundStyle(Tokens.Palette.text)
            .accessibilityAddTraits(.isHeader)
        if let status = CouchLabels.status(view) {
            Text(status)
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.muted)
        }
        if view.role == .host, let code = view.code, let share = view.shareUrl {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .center, spacing: Tokens.Spacing.xxl) { invite(code: code, share: share) }
                VStack(alignment: .leading, spacing: Tokens.Spacing.xl) { invite(code: code, share: share) }
            }
        }
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            ForEach(Self.seated(view.members), id: \.id) { member in
                CouchMemberRow(member: member, hostAway: view.hostAway)
            }
        }
        Button(role: .destructive) {
            if view.role == .host {
                ending = true
            } else {
                core.send(.couchLeft)
                onLeft()
            }
        } label: {
            ActionLabel(
                view.role == .host ? L10n.couchEndSession : L10n.couchLeave,
                systemImage: view.role == .host ? "xmark.circle" : "rectangle.portrait.and.arrow.right")
        }
        .secondaryAction()
        .fixedSize()
        .accessibilityIdentifier("couch-end")
        // on TV this is often the only button in reach: ending it for everyone takes a second yes
        .confirmationDialog(L10n.couchEndConfirm, isPresented: $ending, titleVisibility: .visible) {
            Button(L10n.couchEndSession, role: .destructive) {
                core.send(.couchEndRequested)
                onLeft()
            }
        }
    }

    /// The QR code of the join page beside the code itself, both large enough to read across a room.
    @ViewBuilder private func invite(code: String, share: String) -> some View {
        QRCodeView(share)
            .frame(width: qrSize, height: qrSize)
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            Text(L10n.couchShareLabel)
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.muted)
                .fixedSize(horizontal: false, vertical: true)
            Text(CouchLabels.spaced(code))
                .font(.system(size: codeSize, weight: .bold, design: .monospaced))
                .foregroundStyle(Tokens.Palette.text)
                .minimumScaleFactor(0.5)
                .lineLimit(1)
                .speechSpellsOutCharacters()
                .accessibilityLabel(L10n.couchCode(code: code))
                .accessibilityIdentifier("couch-code")
            Text(PairingPanel.displayed(share))
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.muted)
                .lineLimit(2)
        }
    }

    private var startButton: some View {
        Button {
            core.send(.couchStartRequested)
        } label: {
            ActionLabel(L10n.couchStartSession, systemImage: "sofa")
        }
        .primaryAction()
        .fixedSize()
    }

    /// The host first, then everyone in the order they sat down.
    static func seated(_ members: [CouchMember]) -> [CouchMember] {
        members.filter(\.host) + members.filter { !$0.host }
    }
}

/// A member on the couch: avatar, name and badges; a member who paused for themselves is dimmed.
struct CouchMemberRow: View {
    let member: CouchMember
    let hostAway: Bool

    var body: some View {
        let badges = CouchLabels.badges(member, hostAway: hostAway)
        HStack(spacing: Tokens.Spacing.md) {
            AvatarView(url: member.avatar?.url, seed: member.seed, name: member.displayName)
                .frame(width: Idiom.isTV ? 64 : 40, height: Idiom.isTV ? 64 : 40)
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                Text(member.displayName)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.text)
                if !badges.isEmpty {
                    Text(badges)
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.muted)
                }
            }
        }
        .opacity(member.paused || (member.host && hostAway) ? 0.55 : 1)
        .accessibilityElement(children: .combine)
    }
}

/// The panel on its own: a sheet over the player or Settings on touch devices, a full-screen
/// panel over the playing video on TV.
struct CouchPanelScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.dismiss) private var dismiss
    @FocusState private var doneFocused: Bool

    var body: some View {
        #if os(tvOS)
            ScrollView {
                VStack(alignment: .leading, spacing: Tokens.Spacing.xxl) {
                    CouchPanel(view: core.couch) { dismiss() }
                    Button(L10n.commonDone) { dismiss() }
                        .primaryAction()
                        .fixedSize()
                        .focused($doneFocused)
                }
                .padding(.horizontal, 120)
                .padding(.vertical, 80)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .scrollClipDisabled()
            // a stray click must not end the session for everyone
            .defaultFocus($doneFocused, true)
            .background(Tokens.Palette.bg.opacity(0.88))
            .presentationBackground(.clear)
        #else
            NavigationStack {
                ScrollView {
                    CouchPanel(view: core.couch) { dismiss() }
                        .padding(Tokens.Spacing.xl)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .scrollBounceBehavior(.basedOnSize)
                .background(Tokens.Palette.bg)
                .navigationTitle(L10n.couchParticipantsTitle)
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .confirmationAction) {
                        Button(L10n.commonDone) { dismiss() }
                    }
                }
            }
            .presentationDetents([.medium, .large])
        #endif
    }
}
