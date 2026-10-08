import AVFoundation
import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The app's root: shows what `AppView.phase` asks for, keeps the display language and accent in
/// step with the session, opens `couchverse://` links, reports returns to the foreground and shows
/// the core's notices and, over everything, the player.
public struct CouchverseRoot: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.scenePhase) private var scenePhase
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var choreography = ProfileChoreography()
    @State private var pickingProfile = false
    @State private var switchingAccount = false
    @State private var approving = false
    @State private var connecting = false
    /// Absent in previews and snapshots, which never play.
    @Environment(PlayerController.self) private var player: PlayerController?

    public init() {}

    public var body: some View {
        ZStack {
            Tokens.Palette.bg.ignoresSafeArea()
            phaseContent
                .modifier(RiseFromBlack(choreography: choreography))
                .disabled(pickingProfile)
            if pickingProfile {
                WhosWatchingScreen { pickingProfile = false }
                    .transition(.opacity)
            }
            ProfileChoreographyOverlay(choreography: choreography)
            if connecting && core.signIn.status == .loading {
                ConnectingBanner()
                    .transition(.move(edge: .top).combined(with: .opacity))
            }
            NoticeToasts()
        }
        .animation(reduceMotion ? .easeInOut(duration: 0.3) : Tokens.Motion.smooth, value: core.app.phase)
        .animation(reduceMotion ? .easeInOut(duration: 0.3) : Tokens.Motion.smooth, value: pickingProfile)
        .animation(Tokens.Motion.smooth, value: core.app.activeAccount)
        .environment(choreography)
        .environment(\.showProfilePicker, RootAction(name: "profile-picker") { pickingProfile = true })
        .environment(\.showAccountSwitcher, RootAction(name: "account-switcher") { switchingAccount = true })
        .accent(core.session.accent)
        .environment(\.locale, L10n.locale)
        .preferredColorScheme(.dark)
        .onChange(of: core.session.language, initial: true) { _, language in L10n.language = language }
        .onChange(of: scenePhase) { _, phase in
            if phase == .active {
                core.send(.appBecameActive)
            }
        }
        .onChange(of: core.signIn.status) { _, status in
            if status != .loading {
                connecting = false
            }
        }
        .onOpenURL(perform: open)
        .sheet(isPresented: $switchingAccount) { AccountSwitcherSheet() }
        .modal(isPresented: $approving) { ApproveDeviceScreen(openedFromLink: true) }
        .fullScreenCover(isPresented: playing) {
            if let player {
                PlayerScreen()
                    .environment(core)
                    .environment(player)
                    .accent(core.session.accent)
                    .environment(\.locale, L10n.locale)
            }
        }
        .task(id: player == nil) {
            guard player != nil else { return }
            reportCapabilities()
            // a receiver or AirPods can bring Atmos, or take it away
            for await _ in NotificationCenter.default.notifications(named: AVAudioSession.routeChangeNotification) {
                reportCapabilities()
            }
        }
    }

    /// Up while the core has something playing; dismissing it (a swipe, the remote's Back)
    /// closes the player in the core.
    private var playing: Binding<Bool> {
        Binding(
            get: { player != nil && core.player.target != nil },
            set: { presented in
                if !presented && core.player.target != nil {
                    core.send(.playerClosed)
                }
            })
    }

    private func reportCapabilities() {
        let measured = DeviceCapabilities.measure(screen: DeviceCapabilities.currentScreen)
        core.send(.capabilitiesReported(.avPlayer(measured)))
    }

    @ViewBuilder private var phaseContent: some View {
        switch core.app.phase {
        case .starting:
            LaunchView()
        case .welcome, .signIn:
            OnboardingFlow(startsAtWelcome: core.app.phase == .welcome)
        case .chooseAccount:
            WhosWatchingScreen()
        case .ready:
            MainTabs()
                // a switch on a phone cross-fades the whole app to the new account
                .id(core.app.activeAccount)
                .transition(.opacity.combined(with: .scale(scale: 0.98)))
        }
    }

    private func open(_ url: URL) {
        guard let link = DeepLink(url) else { return }
        switch link {
        case .approve:
            approving = true
        case .connect:
            connecting = true
        }
        core.send(.linkOpened(Link(url: url.absoluteString)))
    }
}

/// The first frames, while the core reads what it persisted.
struct LaunchView: View {
    @Environment(\.accent) private var accent

    var body: some View {
        ZStack {
            GlowBackdrop(tint: accent.color, intensity: 0.8)
            Image(systemName: "sofa.fill")
                .font(.system(size: Idiom.isTV ? 140 : 72, weight: .semibold))
                .foregroundStyle(accent.color)
                .shadow(color: accent.color.opacity(0.6), radius: 30)
                .accessibilityLabel("Couchverse")
        }
    }
}

/// Signing in with a scanned or tapped connect link, over whatever is showing.
private struct ConnectingBanner: View {
    var body: some View {
        VStack {
            HStack(spacing: Tokens.Spacing.md) {
                ProgressView()
                Text(L10n.onboardingConnecting)
                    .typeRole(Tokens.TypeRamp.card)
            }
            .padding(.horizontal, Tokens.Spacing.xl)
            .padding(.vertical, Tokens.Spacing.md)
            .glassEffect(.regular, in: Capsule())
            .padding(.top, Tokens.Spacing.xl)
            Spacer()
        }
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.updatesFrequently)
    }
}
