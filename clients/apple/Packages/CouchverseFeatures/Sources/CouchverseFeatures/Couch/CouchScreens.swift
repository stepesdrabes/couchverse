import CouchverseCore
import CouchverseDesign
import SwiftUI

/// A follower with nothing to play yet: the host is choosing what to watch or stepped away, or the
/// couch is still connecting. The player takes over as soon as the host plays something.
struct CouchWaitingScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accent) private var accent

    var body: some View {
        let view = core.couch
        ZStack {
            GlowBackdrop(tint: accent.color, intensity: 0.7)
            VStack(spacing: Idiom.isTV ? Tokens.Spacing.xxl : Tokens.Spacing.xl) {
                Image(systemName: "sofa.fill")
                    .font(.system(size: Idiom.isTV ? 120 : 64, weight: .semibold))
                    .foregroundStyle(accent.color)
                    .shadow(color: accent.color.opacity(0.6), radius: 30)
                    .accessibilityHidden(true)
                Text(CouchLabels.status(view) ?? L10n.couchLoading)
                    .typeRole(Tokens.TypeRamp.section)
                    .foregroundStyle(Tokens.Palette.text)
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityAddTraits(.updatesFrequently)
                CouchAvatars(members: view.members, size: Idiom.isTV ? 72 : 40)
                Button {
                    core.send(.couchLeft)
                } label: {
                    ActionLabel(L10n.couchLeave, systemImage: "rectangle.portrait.and.arrow.right")
                }
                .secondaryAction()
                .accessibilityIdentifier("couch-leave")
                .fixedWidthIfItFits()
            }
            .padding(Tokens.Spacing.xl)
            .readableWidth(Idiom.isTV ? 1000 : 520)
        }
    }
}

/// The session this device watched or steered is over: why, for a moment, before the cover goes.
struct CouchEndedScreen: View {
    let done: () -> Void
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accent) private var accent

    var body: some View {
        ZStack {
            GlowBackdrop(tint: accent.color, intensity: 0.5)
            VStack(spacing: Tokens.Spacing.xl) {
                CatalogMessage(systemImage: "sofa", title: CouchLabels.status(core.couch) ?? "", message: nil)
                Button(L10n.commonDone, action: done)
                    .primaryAction()
                    .fixedSize()
            }
        }
        // the cover goes again in a few seconds
        .onAppear {
            if let status = CouchLabels.status(core.couch) {
                AccessibilityNotification.Announcement(status).post()
            }
        }
    }
}

/// A phone steering this account's player on another device (the TV): play and pause, ten-second
/// skips and the episodes either side. Nothing plays here.
struct CouchRemoteScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accent) private var accent
    @ScaledMetric(relativeTo: .largeTitle) private var clockSize: CGFloat = 64

    var body: some View {
        let view = core.couch
        ZStack(alignment: .topLeading) {
            GlowBackdrop(tint: accent.color, intensity: 0.6)
            ScrollView {
                VStack(spacing: Tokens.Spacing.xl) {
                    Text(L10n.couchRemoteTitle)
                        .typeRole(Tokens.TypeRamp.title)
                        .foregroundStyle(Tokens.Palette.text)
                        .accessibilityAddTraits(.isHeader)
                    if let status = CouchLabels.status(view) {
                        Text(status)
                            .typeRole(Tokens.TypeRamp.body)
                            .foregroundStyle(Tokens.Palette.mutedText)
                    }
                    CouchAvatars(members: view.members, size: 40)
                    TimelineView(.periodic(from: .now, by: 0.5)) { _ in
                        Text(CatalogLabels.clock(seconds: UInt64(max(position, 0))))
                            .font(.system(size: clockSize, weight: .semibold, design: .rounded))
                            .monospacedDigit()
                            .lineLimit(1)
                            .minimumScaleFactor(0.5)
                            .foregroundStyle(Tokens.Palette.text)
                            .contentTransition(.numericText())
                            .accessibilityAddTraits(.updatesFrequently)
                    }
                    transport(playing: view.playing)
                    episodes
                    Button {
                        core.send(.couchLeft)
                    } label: {
                        ActionLabel(L10n.couchLeave, systemImage: "rectangle.portrait.and.arrow.right")
                    }
                    .secondaryAction()
                    .fixedWidthIfItFits()
                }
                .padding(.horizontal, Tokens.Spacing.xl)
                .padding(.vertical, 72)
                .readableWidth(520)
            }
            .scrollBounceBehavior(.basedOnSize)
        }
    }

    /// Where the host is now, moved on from its last report while it plays.
    private var position: Double {
        CouchLabels.position(core.couch, now: core.nowMs())
    }

    private func transport(playing: Bool) -> some View {
        HStack(spacing: Tokens.Spacing.xxl) {
            RemoteButton(systemImage: "gobackward.10", label: L10n.playerBack10Seconds) { seek(by: -10) }
            Button {
                command(playing ? .pause : .play)
            } label: {
                Image(systemName: playing ? "pause.fill" : "play.fill")
                    .font(.system(size: 40, weight: .semibold))
                    .frame(width: 88, height: 88)
            }
            .primaryAction()
            .buttonBorderShape(.circle)
            .accessibilityLabel(playing ? L10n.commonPause : L10n.commonPlay)
            .accessibilityIdentifier("remote-play-pause")
            RemoteButton(systemImage: "goforward.10", label: L10n.playerForward10Seconds) { seek(by: 10) }
        }
    }

    private var episodes: some View {
        HStack(spacing: Tokens.Spacing.xxl) {
            RemoteButton(systemImage: "backward.end.fill", label: L10n.couchRemotePrevious) { command(.previous) }
            RemoteButton(systemImage: "forward.end.fill", label: L10n.couchRemoteNext) { command(.next) }
        }
    }

    private func seek(by seconds: Double) {
        core.send(.couchRemoteCommanded(RemoteControl(action: .seek, positionSeconds: max(position + seconds, 0))))
    }

    private func command(_ action: RemoteAction) {
        core.send(.couchRemoteCommanded(RemoteControl(action: action)))
    }
}

private struct RemoteButton: View {
    let systemImage: String
    let label: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemImage)
                .font(.system(size: 24, weight: .semibold))
                .frame(width: 56, height: 56)
        }
        .secondaryAction()
        .buttonBorderShape(.circle)
        .accessibilityLabel(label)
    }
}

/// The members' avatars in a row, the host first, a member who paused for themselves dimmed.
struct CouchAvatars: View {
    let members: [CouchMember]
    let size: CGFloat

    var body: some View {
        HStack(spacing: -size / 5) {
            ForEach(CouchPanel.seated(members), id: \.id) { member in
                AvatarView(url: member.avatar?.url, seed: member.seed, name: member.displayName)
                    .frame(width: size, height: size)
                    .overlay { Circle().strokeBorder(Tokens.Palette.bg, lineWidth: 2) }
                    .opacity(member.paused ? 0.5 : 1)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(L10n.couchOnCouchCount(count: String(members.count)))
        .accessibilityValue(members.map(\.displayName).joined(separator: ", "))
    }
}

/// The TV's Couch tab: the session this TV is on (the host's code to share, who is there), or
/// joining one by code, and how to start one.
struct CouchHubScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accent) private var accent

    var body: some View {
        ScrollView {
            Group {
                if core.couch.isLive {
                    CouchPanel(view: core.couch)
                } else {
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xxl) {
                        JoinCouchForm()
                        Label(L10n.couchStartFromVideo, systemImage: "play.rectangle")
                            .typeRole(Tokens.TypeRamp.body)
                            .foregroundStyle(Tokens.Palette.mutedText)
                    }
                }
            }
            .padding(.horizontal, 80)
            .padding(.vertical, 60)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .scrollClipDisabled()
        .background { GlowBackdrop(tint: accent.color, intensity: 0.5) }
    }
}

/// Settings' way onto a couch: joining one by code, or the session this device is on.
struct CouchSettingsSection: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.showCouchJoin) private var showCouchJoin
    @State private var panel = false

    var body: some View {
        if core.session.features.couch {
            Section {
                if core.couch.isLive {
                    Button {
                        panel = true
                    } label: {
                        Label(L10n.couchOnCouchCount(count: String(core.couch.members.count)), systemImage: "sofa.fill")
                    }
                } else {
                    Button {
                        showCouchJoin()
                    } label: {
                        Label(L10n.couchJoinTitle, systemImage: "sofa")
                    }
                    .accessibilityIdentifier("join-couch")
                }
            }
            .modal(isPresented: $panel) { CouchPanelScreen() }
        }
    }
}
