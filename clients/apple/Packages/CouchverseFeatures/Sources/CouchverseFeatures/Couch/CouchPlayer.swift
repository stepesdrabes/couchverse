import CouchverseCore
import CouchverseDesign
import SwiftUI

/// What everyone on a couch sees over the picture: the reactions floating up and, for a follower,
/// what the host is doing (paused, away, resynced).
struct CouchPlayerOverlay: View {
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        let couch = core.couch
        if core.couchOn && couch.isLive {
            ZStack(alignment: .top) {
                ReactionsOverlay(reactions: couch.reactions)
                CouchStatusPill(view: couch)
                    // below the touch player's top buttons
                    .padding(.top, Idiom.isTV ? 60 : 72)
            }
        }
    }
}

/// Reactions rising from the bottom of the picture and fading.
struct ReactionsOverlay: View {
    let reactions: [Reaction]

    var body: some View {
        GeometryReader { geometry in
            ForEach(reactions, id: \.id) { reaction in
                FloatingReaction(reaction: reaction, area: geometry.size)
            }
        }
        .ignoresSafeArea()
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

private struct FloatingReaction: View {
    let reaction: Reaction
    let area: CGSize
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.ambience) private var ambience
    @State private var risen = false

    var body: some View {
        // a lane from the id, so reactions sent together spread out
        let lane = 0.15 + Double((reaction.id % 70) * 37 % 70) / 100
        let start = area.height - (Idiom.isTV ? 200 : 140)
        // the core takes a reaction away after its 2.2 s rise; Reduce Motion fades it in place
        Text(reaction.emoji)
            .font(.system(size: Idiom.isTV ? 88 : 44))
            .position(x: area.width * lane, y: risen && !reduceMotion ? start - area.height * 0.45 : start)
            .opacity(risen ? 0 : 1)
            .onAppear {
                guard ambience == .live else { return }
                withAnimation(.easeOut(duration: 2.2)) { risen = true }
            }
    }
}

/// The one line about the session over the picture, when it needs saying.
struct CouchStatusPill: View {
    let view: CouchView
    @Environment(\.ambience) private var ambience

    var body: some View {
        if let status = CouchLabels.status(view) {
            Text(status)
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(Tokens.Palette.text)
                .padding(.horizontal, Tokens.Spacing.xl)
                .padding(.vertical, Tokens.Spacing.md)
                // snapshots have no host app to draw glass in
                .background(ambience == .flat ? Tokens.Palette.surface.opacity(0.9) : .clear, in: Capsule())
                .glassEffect(ambience == .flat ? .identity : .regular, in: Capsule())
                .accessibilityAddTraits(.updatesFrequently)
                .transition(.opacity)
                // over a playing video nothing else would tell VoiceOver the host paused
                .onAppear { AccessibilityNotification.Announcement(status).post() }
                .onChange(of: status) { _, status in AccessibilityNotification.Announcement(status).post() }
        }
    }
}

/// Who is on the couch, in the TV player's info panel: the host first, a member paused for
/// themselves dimmed.
struct CouchInfoPanel: View, Equatable {
    let members: [CouchMember]
    let hostAway: Bool

    #if os(tvOS)
        static func controller(_ panel: CouchInfoPanel) -> UIHostingController<CouchInfoPanel> {
            let controller = UIHostingController(rootView: panel)
            controller.title = L10n.couchParticipantsTitle
            controller.preferredContentSize = CGSize(width: 0, height: 300)
            return controller
        }
    #endif

    var body: some View {
        ScrollView(.horizontal) {
            HStack(alignment: .top, spacing: 48) {
                ForEach(CouchPanel.seated(members), id: \.id) { member in
                    let badges = CouchLabels.badges(member, hostAway: hostAway)
                    VStack(spacing: Tokens.Spacing.sm) {
                        AvatarView(url: member.avatar?.url, seed: member.seed, name: member.displayName)
                            .frame(width: 120, height: 120)
                            .accessibilityHidden(true)
                        Text(member.displayName)
                            .typeRole(Tokens.TypeRamp.card)
                            .foregroundStyle(Tokens.Palette.text)
                            .lineLimit(1)
                        if !badges.isEmpty {
                            Text(badges)
                                .typeRole(Tokens.TypeRamp.caption)
                                .foregroundStyle(Tokens.Palette.mutedText)
                                .lineLimit(1)
                        }
                    }
                    .frame(width: 240)
                    .opacity(member.paused || (member.host && hostAway) ? 0.55 : 1)
                    // focus walks a couch wider than the screen
                    .focusable()
                    .hoverEffect(.highlight)
                    .accessibilityElement(children: .combine)
                }
            }
            .padding(.horizontal, 80)
            .padding(.vertical, 30)
        }
        .scrollClipDisabled()
    }
}

/// The couch in the TV player's transport bar: starting a session, its panel (the code to share,
/// who is there) and the way off the couch, and reactions while a session is on. Compared, so the
/// bar's menus are rebuilt only when what they show changes.
struct CouchMenu: Equatable {
    /// This device's role on a live couch; nil with none.
    let role: CouchRole?
    let code: String?
    let members: Int
    let reactions: [String]

    /// Nothing at all while the server has couch sessions off.
    init?(_ view: CouchView, enabled: Bool) {
        guard enabled else { return nil }
        role = view.isLive ? view.role : nil
        code = view.code
        members = view.members.count
        reactions = Reactions.choices(recent: view.recentEmojis)
    }

    /// The couch menu's entries, by title, for tests and the transport bar alike.
    var entries: [String] {
        switch role {
        case nil: [L10n.couchStartSession]
        case .host: [code.map { L10n.couchCode(code: CouchLabels.spaced($0)) } ?? L10n.couchOpen, L10n.couchEndSession]
        case .follower, .remote: [L10n.couchOnCouchCount(count: String(members)), L10n.couchLeave]
        }
    }

    #if os(tvOS)
        func items(send: @escaping (Event) -> Void, panel: @escaping () -> Void) -> [UIMenuElement] {
            let titles = entries
            let actions: [UIAction]
            if let role {
                let symbol = role == .host ? "qrcode" : "person.2"
                let leave: Event = role == .host ? .couchEndRequested : .couchLeft
                actions = [
                    UIAction(title: titles[0], image: UIImage(systemName: symbol)) { _ in panel() },
                    UIAction(title: titles[1], image: UIImage(systemName: "xmark.circle"), attributes: .destructive) {
                        _ in send(leave)
                    },
                ]
            } else {
                actions = [
                    UIAction(title: titles[0], image: UIImage(systemName: "play.circle")) { _ in
                        send(.couchStartRequested)
                        panel()
                    }
                ]
            }
            var items: [UIMenuElement] = [
                UIMenu(title: L10n.couchOpen, image: UIImage(systemName: "sofa"), children: actions)
            ]
            if role != nil {
                items.append(
                    UIMenu(
                        title: L10n.couchReact, image: UIImage(systemName: "face.smiling"),
                        children: reactions.map { emoji in
                            UIAction(title: emoji) { _ in send(.couchEmojiSent(CouchReaction(emoji: emoji))) }
                        }))
            }
            return items
        }
    #endif
}

#if os(iOS)
    /// Over a touch player while a couch session is on: who is on it (the panel) and reactions.
    struct CouchPlayerButtons: View {
        let showCouch: () -> Void
        let onOpen: () -> Void
        @Environment(CoreRuntime.self) private var core
        @State private var reacting = false

        var body: some View {
            let couch = core.couch
            Button {
                onOpen()
                showCouch()
            } label: {
                Label(String(couch.members.count), systemImage: "sofa.fill")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(.white)
                    .frame(height: 30)
            }
            .buttonStyle(.glass)
            .accessibilityLabel(L10n.couchOnCouchCount(count: String(couch.members.count)))
            .accessibilityShowsLargeContentViewer {
                Label(L10n.couchOnCouchCount(count: String(couch.members.count)), systemImage: "sofa.fill")
            }
            .accessibilityIdentifier("player-couch")
            Button {
                onOpen()
                reacting = true
            } label: {
                Image(systemName: "face.smiling")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(.white)
                    .frame(width: 30, height: 30)
            }
            .buttonStyle(.glass)
            .buttonBorderShape(.circle)
            .accessibilityLabel(L10n.couchReact)
            .accessibilityShowsLargeContentViewer { Label(L10n.couchReact, systemImage: "face.smiling") }
            .accessibilityIdentifier("player-react")
            .popover(isPresented: $reacting) {
                ReactionBar(choices: Reactions.choices(recent: couch.recentEmojis)) { emoji in
                    core.send(.couchEmojiSent(CouchReaction(emoji: emoji)))
                }
                .presentationCompactAdaptation(.popover)
            }
        }
    }

    /// The reactions to send, what the viewer sent lately first; it stays up for a few in a row.
    /// "More" brings up the system emoji keyboard for any other.
    struct ReactionBar: View {
        let choices: [String]
        let send: (String) -> Void
        @State private var typing = false

        var body: some View {
            LazyVGrid(columns: Array(repeating: GridItem(.fixed(48), spacing: Tokens.Spacing.xs), count: 6)) {
                ForEach(choices, id: \.self) { emoji in
                    Button {
                        send(emoji)
                    } label: {
                        Text(emoji)
                            .font(.system(size: 30))
                            .frame(width: 48, height: 48)
                    }
                    .buttonStyle(.plain)
                }
                Button {
                    typing.toggle()
                } label: {
                    Image(systemName: typing ? "keyboard.chevron.compact.down" : "ellipsis")
                        .font(.system(size: 22, weight: .semibold))
                        .foregroundStyle(Tokens.Palette.text)
                        .frame(width: 48, height: 48)
                        .background(Tokens.Palette.surface2, in: Circle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(L10n.couchReactMore)
                .accessibilityIdentifier("couch-react-more")
            }
            .padding(Tokens.Spacing.md)
            .background { EmojiKeyboard(typing: $typing, send: send).frame(width: 0, height: 0) }
            // a named group, so the label does not replace each emoji's own
            .accessibilityElement(children: .contain)
            .accessibilityLabel(L10n.couchReact)
        }
    }

    /// An invisible field that holds the system emoji keyboard up while `typing` (the plain
    /// keyboard for a viewer without the emoji one) and sends each emoji typed as a reaction.
    private struct EmojiKeyboard: UIViewRepresentable {
        @Binding var typing: Bool
        let send: (String) -> Void

        func makeUIView(context: Context) -> EmojiField {
            let field = EmojiField()
            field.tintColor = .clear
            field.delegate = context.coordinator
            field.addTarget(context.coordinator, action: #selector(Coordinator.typed(_:)), for: .editingChanged)
            return field
        }

        func updateUIView(_ field: EmojiField, context: Context) {
            context.coordinator.parent = self
            if typing && !field.isFirstResponder {
                field.becomeFirstResponder()
            } else if !typing && field.isFirstResponder {
                field.resignFirstResponder()
            }
        }

        func makeCoordinator() -> Coordinator { Coordinator(parent: self) }

        final class Coordinator: NSObject, UITextFieldDelegate {
            var parent: EmojiKeyboard

            init(parent: EmojiKeyboard) {
                self.parent = parent
            }

            @objc func typed(_ field: UITextField) {
                Reactions.emoji(in: field.text ?? "").forEach(parent.send)
                field.text = ""
            }

            func textFieldDidEndEditing(_ textField: UITextField) {
                parent.typing = false
            }
        }
    }

    final class EmojiField: UITextField {
        // UIKit offers no emoji keyboard type: the field asks for the emoji input mode, which the
        // system uses while the viewer has that keyboard
        override var textInputMode: UITextInputMode? {
            UITextInputMode.activeInputModes.first { $0.primaryLanguage == "emoji" } ?? super.textInputMode
        }
    }

    /// The couch in the touch player's options menu: starting a session, then the code to share and
    /// ending it.
    struct CouchOptions: View {
        let showCouch: () -> Void
        @Environment(CoreRuntime.self) private var core

        var body: some View {
            if let menu = CouchMenu(core.couch, enabled: core.couchOn) {
                let titles = menu.entries
                Section {
                    if menu.role == nil {
                        Button {
                            core.send(.couchStartRequested)
                            showCouch()
                        } label: {
                            Label(titles[0], systemImage: "sofa")
                        }
                    } else if menu.role == .host {
                        Button(action: showCouch) {
                            Label(titles[0], systemImage: "qrcode")
                        }
                        Button(role: .destructive) {
                            core.send(.couchEndRequested)
                        } label: {
                            Label(titles[1], systemImage: "xmark.circle")
                        }
                    }
                }
            }
        }
    }
#endif
