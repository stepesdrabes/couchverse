import AVKit
import CouchverseCore
import CouchverseDesign
import SwiftUI

/// `AVPlayerViewController` for SwiftUI (`VideoPlayer` has none of the hooks below). On TV the
/// core's options go into the transport bar, the next episode into a contextual action and the
/// episodes into an info panel; on touch devices a tap on the picture also toggles the overlay.
struct SystemPlayer: UIViewControllerRepresentable {
    let controller: PlayerController
    let view: PlayerView
    let nativeAudio: Bool
    let nativeSubtitles: Bool
    let linear: Bool
    let send: (Event) -> Void
    var onTap: (() -> Void)?
    /// The couch's transport bar menus on TV; nil while the server has couch sessions off.
    var couch: CouchMenu?
    var onCouchPanel: () -> Void = {}

    func makeCoordinator() -> Coordinator { Coordinator() }

    func makeUIViewController(context: Context) -> AVPlayerViewController {
        let player = AVPlayerViewController()
        player.player = controller.player
        player.delegate = context.coordinator
        #if os(iOS)
            player.allowsPictureInPicturePlayback = true
            player.canStartPictureInPictureAutomaticallyFromInline = true
            player.updatesNowPlayingInfoCenter = true
            player.entersFullScreenWhenPlaybackBegins = false
            player.exitsFullScreenWhenPlaybackEnds = false
            let tap = UITapGestureRecognizer(target: context.coordinator, action: #selector(Coordinator.tapped))
            tap.cancelsTouchesInView = false
            tap.delegate = context.coordinator
            player.view.addGestureRecognizer(tap)
        #else
            // the TV switches to the content's frame rate and dynamic range
            player.appliesPreferredDisplayCriteriaAutomatically = true
        #endif
        return player
    }

    func updateUIViewController(_ player: AVPlayerViewController, context: Context) {
        context.coordinator.onTap = onTap
        if player.requiresLinearPlayback != linear {
            player.requiresLinearPlayback = linear
        }
        #if os(tvOS)
            context.coordinator.update(
                player, view: view, nativeAudio: nativeAudio, nativeSubtitles: nativeSubtitles, couch: couch,
                send: send, couchPanel: onCouchPanel)
        #endif
    }

    final class Coordinator: NSObject, AVPlayerViewControllerDelegate, UIGestureRecognizerDelegate {
        var onTap: (() -> Void)?

        @objc func tapped() {
            onTap?()
        }

        func gestureRecognizer(
            _ gestureRecognizer: UIGestureRecognizer,
            shouldRecognizeSimultaneouslyWith otherGestureRecognizer: UIGestureRecognizer
        ) -> Bool {
            true
        }

        #if os(tvOS)
            private var menus: TransportMenus?
            private var next: NextUp?
            private var seasons: [PlayerSeason]?

            /// Replaces the system's items only when what they show changed: a menu rebuilt
            /// under the viewer's finger would close.
            func update(
                _ player: AVPlayerViewController, view: PlayerView, nativeAudio: Bool, nativeSubtitles: Bool,
                couch: CouchMenu?, send: @escaping (Event) -> Void, couchPanel: @escaping () -> Void
            ) {
                let menus = TransportMenus(
                    view: view, nativeAudio: nativeAudio, nativeSubtitles: nativeSubtitles, couch: couch)
                if menus != self.menus {
                    self.menus = menus
                    player.transportBarCustomMenuItems = menus.items(send: send, couchPanel: couchPanel)
                }
                if view.nextUp != next {
                    next = view.nextUp
                    player.contextualActions =
                        view.nextUp.map { next in
                            [
                                UIAction(
                                    title: L10n.playerUpNext(seconds: String(next.countdownSeconds)),
                                    image: UIImage(systemName: next.shuffled ? "shuffle" : "forward.end.fill")
                                ) { _ in send(.nextEpisodeRequested) }
                            ]
                        } ?? []
                }
                if view.seasons != seasons {
                    seasons = view.seasons
                    player.customInfoViewControllers =
                        view.seasons.isEmpty ? [] : [EpisodesPanel.controller(view.seasons, send: send)]
                }
            }
        #endif
    }
}

#if os(tvOS)
    /// The transport bar's own items: quality, the core's audio and subtitles when the system's
    /// menus cannot list them (another language's file, a sidecar file), shuffle, and the couch.
    struct TransportMenus: Equatable {
        let qualities: [QualityOption]
        let quality: String
        let audio: [TrackOption]
        let audioSelected: String?
        let subtitles: [TrackOption]
        let subtitleSelected: String?
        let shuffle: Bool?
        let couch: CouchMenu?

        init(view: PlayerView, nativeAudio: Bool, nativeSubtitles: Bool, couch: CouchMenu? = nil) {
            qualities = view.qualities.count > 1 ? view.qualities : []
            quality = view.quality
            audio = !nativeAudio && view.audio.count > 1 ? view.audio : []
            audioSelected = view.audioSelected
            subtitles = nativeSubtitles ? [] : view.subtitles
            subtitleSelected = view.subtitleSelected
            shuffle = view.shuffleAvailable ? view.shuffle : nil
            self.couch = couch
        }

        func items(send: @escaping (Event) -> Void, couchPanel: @escaping () -> Void = {}) -> [UIMenuElement] {
            var items: [UIMenuElement] = []
            if !qualities.isEmpty {
                items.append(
                    UIMenu(
                        title: L10n.playerQuality, image: UIImage(systemName: "slider.horizontal.3"),
                        options: .singleSelection,
                        children: qualities.map { option in
                            UIAction(title: PlayerLabels.quality(option), state: option.key == quality ? .on : .off) {
                                _ in
                                send(.qualityChosen(QualityChoice(key: option.key)))
                            }
                        }))
            }
            if !audio.isEmpty {
                items.append(
                    UIMenu(
                        title: L10n.playerAudio, image: UIImage(systemName: "waveform"), options: .singleSelection,
                        children: audio.map { track in
                            UIAction(title: track.label, state: track.id == audioSelected ? .on : .off) { _ in
                                send(.audioChosen(TrackChoice(id: track.id)))
                            }
                        }))
            }
            if !subtitles.isEmpty {
                let off = UIAction(title: L10n.playerSubtitleOff, state: subtitleSelected == nil ? .on : .off) { _ in
                    send(.subtitlesChosen(TrackChoice(id: nil)))
                }
                items.append(
                    UIMenu(
                        title: L10n.playerSubtitles, image: UIImage(systemName: "captions.bubble"),
                        options: .singleSelection,
                        children: [off]
                            + subtitles.map { track in
                                UIAction(title: track.label, state: track.id == subtitleSelected ? .on : .off) { _ in
                                    send(.subtitlesChosen(TrackChoice(id: track.id)))
                                }
                            }))
            }
            if let shuffle {
                items.append(
                    UIAction(
                        title: L10n.playerShuffle, image: UIImage(systemName: "shuffle"), state: shuffle ? .on : .off
                    ) { _ in send(.shuffleToggled) })
            }
            if let couch {
                items += couch.items(send: send, panel: couchPanel)
            }
            return items
        }
    }

    /// The series' episodes in the info panel, by season, the current one marked.
    struct EpisodesPanel: View {
        let seasons: [PlayerSeason]
        let send: (Event) -> Void

        static func controller(_ seasons: [PlayerSeason], send: @escaping (Event) -> Void) -> UIViewController {
            let controller = UIHostingController(rootView: EpisodesPanel(seasons: seasons, send: send))
            controller.title = L10n.playerEpisodes
            controller.preferredContentSize = CGSize(width: 0, height: 380)
            return controller
        }

        var body: some View {
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 40) {
                    ForEach(seasons, id: \.number) { season in
                        ForEach(season.episodes, id: \.id) { episode in
                            Button {
                                send(.playRequested(PlayTarget(kind: .episode, id: episode.id)))
                            } label: {
                                ArtworkImage(image: episode.still)
                                    .frame(width: 360, height: 202)
                                    .overlay(alignment: .bottomLeading) {
                                        Text(PlayerLabels.episode(season: season.number, episode))
                                            .font(.caption.weight(.semibold))
                                            .foregroundStyle(.white)
                                            .lineLimit(1)
                                            .padding(Tokens.Spacing.md)
                                            .frame(maxWidth: .infinity, alignment: .leading)
                                            .background(
                                                .linearGradient(
                                                    colors: [.clear, .black.opacity(0.8)], startPoint: .top,
                                                    endPoint: .bottom))
                                    }
                                    .overlay {
                                        if episode.current {
                                            RoundedRectangle(cornerRadius: Tokens.Radius.card).strokeBorder(
                                                .white, lineWidth: 4)
                                        }
                                    }
                            }
                            .buttonStyle(.card)
                            .accessibilityLabel(PlayerLabels.episode(season: season.number, episode))
                            .accessibilityAddTraits(episode.current ? .isSelected : [])
                        }
                    }
                }
                .padding(.horizontal, 80)
                .padding(.vertical, 30)
            }
            .scrollClipDisabled()
        }
    }
#endif
