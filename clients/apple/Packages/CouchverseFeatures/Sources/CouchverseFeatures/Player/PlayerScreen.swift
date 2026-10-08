import AVKit
import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The player, over everything while the core has something playing: the system player with
/// its own transport, scrubbing, PiP and AirPlay, plus what only Couchverse knows (quality, the
/// core's tracks, episodes, shuffle, the next episode, the couch) in the system's extension points
/// on TV and a light overlay on touch devices. Closing it tells the core, which saves progress and
/// frees a transcode.
struct PlayerScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(PlayerController.self) private var controller
    @State private var chrome = ChromeVisibility()
    @State private var couchPanel = false

    var body: some View {
        let view = core.player
        ZStack {
            Color.black.ignoresSafeArea()
            SystemPlayer(
                controller: controller, view: view, nativeAudio: controller.nativeAudio,
                nativeSubtitles: controller.nativeSubtitles, linear: controller.linear,
                send: { core.send($0) }, onTap: { chrome.touched() },
                couch: CouchMenu(core.couch, enabled: core.couchOn),
                seats: core.couch.isLive
                    ? CouchInfoPanel(members: core.couch.members, hostAway: core.couch.hostAway) : nil,
                onCouchPanel: { couchPanel = true }
            )
            .ignoresSafeArea()
            .opacity(view.status == .loaded || view.status == .stale ? 1 : 0)
            if let caption = controller.caption {
                CaptionView(text: caption)
            }
            CouchPlayerOverlay()
            switch view.status {
            case .loading, .idle:
                PlayerWaiting(view: view, close: close)
            case .failed, .notFound:
                PlayerProblem(view: view, close: close)
            default:
                #if os(iOS)
                    PlayerChrome(view: view, chrome: chrome, showCouch: { couchPanel = true })
                #else
                    EmptyView()
                #endif
            }
        }
        .preferredColorScheme(.dark)
        .modal(isPresented: $couchPanel) { CouchPanelScreen() }
        .onChange(of: view.audio, initial: true) { _, audio in controller.audioTracks = audio }
        // the remote's Back or the system player's close dismisses the cover, which closes the
        // player in the core (see `AppCover`)
        #if os(iOS)
            .persistentSystemOverlays(.hidden)
            .statusBarHidden()
        #endif
    }

    private func close() {
        core.closePlayer()
    }
}

/// While the core fetches the playback, starts a transcode or waits for one being prepared.
private struct PlayerWaiting: View {
    let view: PlayerView
    let close: () -> Void

    var body: some View {
        ZStack {
            ArtworkImage(image: view.backdrop)
                .ignoresSafeArea()
                .opacity(0.35)
            VStack(spacing: Tokens.Spacing.lg) {
                if let percent = view.preparing {
                    Text(L10n.playerPreparingTitle)
                        .typeRole(Tokens.TypeRamp.section)
                        .foregroundStyle(Tokens.Palette.text)
                    Text(L10n.playerPreparingDescription)
                        .typeRole(Tokens.TypeRamp.body)
                        .foregroundStyle(Tokens.Palette.mutedText)
                        .multilineTextAlignment(.center)
                    ProgressView(value: Double(min(percent, 100)), total: 100)
                        .frame(maxWidth: Idiom.isTV ? 600 : 280)
                        .accessibilityValue(
                            (Double(min(percent, 100)) / 100).formatted(.percent.locale(L10n.locale)))
                } else {
                    ProgressView()
                        .controlSize(.large)
                        .accessibilityLabel(L10n.commonLoading)
                    if !view.title.isEmpty {
                        Text([view.title, view.subtitle].filter { !$0.isEmpty }.joined(separator: " \u{00B7} "))
                            .typeRole(Tokens.TypeRamp.card)
                            .foregroundStyle(Tokens.Palette.mutedText)
                    }
                }
            }
            .padding(CardMetrics.edge)
            .frame(maxWidth: Idiom.isTV ? 1000 : 520)
            #if os(iOS)
                CloseButton(action: close)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
                    .padding(Tokens.Spacing.lg)
            #endif
        }
    }
}

/// Nothing this device can play, or the player gave up twice.
private struct PlayerProblem: View {
    let view: PlayerView
    let close: () -> Void

    var body: some View {
        let unsupported = view.problem?.code == "unsupported"
        VStack(spacing: Tokens.Spacing.lg) {
            Image(systemName: unsupported ? "film.stack" : "exclamationmark.triangle")
                .font(.system(size: Idiom.isTV ? 80 : 48, weight: .light))
                .foregroundStyle(Tokens.Palette.faintText)
                .accessibilityHidden(true)
            Text(unsupported ? L10n.playerUnsupportedTitle : L10n.playerFailedTitle)
                .typeRole(Tokens.TypeRamp.section)
                .foregroundStyle(Tokens.Palette.text)
                .multilineTextAlignment(.center)
            Text(unsupported ? L10n.playerUnsupportedDescription : (view.problem?.message ?? L10n.problemGeneric))
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.mutedText)
                .multilineTextAlignment(.center)
            Button(action: close) {
                Label(L10n.playerBackToTitle, systemImage: "chevron.backward")
            }
            .primaryAction()
            .fixedSize()
        }
        .padding(CardMetrics.edge)
        .frame(maxWidth: Idiom.isTV ? 1000 : 520)
        .accessibilityElement(children: .contain)
    }
}

/// A sidecar subtitle line, drawn the way the system draws its own.
private struct CaptionView: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.system(size: Idiom.isTV ? 48 : 20, weight: .semibold))
            .foregroundStyle(.white)
            .multilineTextAlignment(.center)
            .padding(.horizontal, Tokens.Spacing.md)
            .padding(.vertical, Tokens.Spacing.xs)
            .background(.black.opacity(0.6), in: RoundedRectangle(cornerRadius: 6))
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
            .padding(.bottom, Idiom.isTV ? 140 : 96)
            .padding(.horizontal, CardMetrics.edge)
            .allowsHitTesting(false)
            .accessibilityHidden(true)
    }
}

struct CloseButton: View {
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: "xmark")
                .font(.system(size: 17, weight: .semibold))
                .frame(width: 44, height: 44)
        }
        .buttonStyle(.glass)
        .buttonBorderShape(.circle)
        .accessibilityLabel(L10n.playerBackToTitle)
        .accessibilityIdentifier("player-close")
    }
}

enum PlayerLabels {
    static func quality(_ option: QualityOption) -> String {
        switch option.kind {
        case .original: L10n.playerQualityOriginal
        case .auto: L10n.playerQualityAuto
        case .rendition: option.height.map { "\($0)p" } ?? option.key
        }
    }

    static func episode(season: UInt32, _ episode: PlayerEpisode) -> String {
        let name = episode.name.isEmpty ? L10n.playerEpisodeNumber(number: String(episode.number)) : episode.name
        return "S\(season) E\(episode.number) \u{00B7} \(name)"
    }

    static func nextUp(_ next: NextUp) -> String {
        "S\(next.season) E\(next.episode) \u{00B7} \(next.name)"
    }
}
