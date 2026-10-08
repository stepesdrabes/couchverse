import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Whether the overlay is up, roughly in step with the system's controls, whose visibility is
/// not observable: any touch shows it, and while playing it fades after a few seconds as they do.
/// A touch shows rather than toggles, so a tap on one of the system's buttons never hides it.
@Observable
final class ChromeVisibility {
    private(set) var visible = true
    @ObservationIgnored private var hide: Task<Void, Never>?
    @ObservationIgnored private var hiding = true

    func touched() {
        keep(hiding: hiding)
    }

    /// Shows it, fading it out later unless playback is paused.
    func keep(hiding: Bool) {
        self.hiding = hiding
        visible = true
        hide?.cancel()
        guard hiding else { return }
        hide = Task {
            try? await Task.sleep(for: .seconds(3))
            if !Task.isCancelled {
                visible = false
            }
        }
    }
}

#if os(iOS)
    /// What the system player cannot show on a phone or tablet: closing back to the title, the
    /// title, and an options menu with quality, the core's tracks, the episodes, shuffle and the
    /// couch; who is on the couch and reactions while a session is on; the next episode's
    /// countdown near the end.
    struct PlayerChrome: View {
        let view: PlayerView
        let chrome: ChromeVisibility
        let showCouch: () -> Void
        @Environment(CoreRuntime.self) private var core
        @Environment(PlayerController.self) private var controller
        @Environment(\.accessibilityReduceMotion) private var reduceMotion

        var body: some View {
            ZStack {
                // the system's controls bring close, AirPlay, PiP, the title, the scrubber and
                // their own audio and subtitle menu; this sits between their top corners
                if chrome.visible {
                    HStack(spacing: Tokens.Spacing.sm) {
                        if core.couchOn && core.couch.isLive {
                            CouchPlayerButtons(showCouch: showCouch, onOpen: { chrome.keep(hiding: false) })
                        }
                        if !view.linear {
                            PlayerOptions(
                                view: view, nativeAudio: controller.nativeAudio,
                                nativeSubtitles: controller.nativeSubtitles, showCouch: showCouch,
                                onOpen: { chrome.keep(hiding: false) })
                        }
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
                    .transition(.opacity)
                }
                if let next = view.nextUp {
                    NextUpCard(next: next)
                        .padding(.trailing, Tokens.Spacing.xl)
                        .padding(.bottom, 170)
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
                        .transition(reduceMotion ? .opacity : .move(edge: .trailing).combined(with: .opacity))
                }
            }
            .animation(Tokens.Motion.standard, value: chrome.visible)
            .motion(Tokens.Motion.smooth, value: view.nextUp?.target)
            .onChange(of: controller.playing, initial: true) { _, playing in chrome.keep(hiding: playing) }
        }
    }

    /// Quality, the audio and subtitles the system's menu does not list, episodes, shuffle and the
    /// couch.
    struct PlayerOptions: View {
        let view: PlayerView
        let nativeAudio: Bool
        let nativeSubtitles: Bool
        let showCouch: () -> Void
        let onOpen: () -> Void
        @Environment(CoreRuntime.self) private var core

        var body: some View {
            Menu {
                if view.qualities.count > 1 {
                    Picker(selection: choice(view.quality) { core.send(.qualityChosen(QualityChoice(key: $0))) }) {
                        ForEach(view.qualities, id: \.key) { option in
                            Text(PlayerLabels.quality(option)).tag(option.key)
                        }
                    } label: {
                        Label(L10n.playerQuality, systemImage: "slider.horizontal.3")
                    }
                    .pickerStyle(.menu)
                }
                if !nativeAudio && view.audio.count > 1 {
                    Picker(selection: choice(view.audioSelected ?? "") { core.send(.audioChosen(TrackChoice(id: $0))) })
                    {
                        ForEach(view.audio, id: \.id) { track in
                            Text(track.label).tag(track.id)
                        }
                    } label: {
                        Label(L10n.playerAudio, systemImage: "waveform")
                    }
                    .pickerStyle(.menu)
                }
                if !nativeSubtitles && !view.subtitles.isEmpty {
                    Picker(
                        selection: choice(view.subtitleSelected ?? "") {
                            core.send(.subtitlesChosen(TrackChoice(id: $0.isEmpty ? nil : $0)))
                        }
                    ) {
                        Text(L10n.playerSubtitleOff).tag("")
                        ForEach(view.subtitles, id: \.id) { track in
                            Text(track.label).tag(track.id)
                        }
                    } label: {
                        Label(L10n.playerSubtitles, systemImage: "captions.bubble")
                    }
                    .pickerStyle(.menu)
                }
                if !view.seasons.isEmpty {
                    Menu {
                        ForEach(view.seasons, id: \.number) { season in
                            Section(L10n.catalogSeasonNumber(number: String(season.number))) {
                                ForEach(season.episodes, id: \.id) { episode in
                                    Button {
                                        core.send(.playRequested(PlayTarget(kind: .episode, id: episode.id)))
                                    } label: {
                                        if episode.current {
                                            Label(
                                                PlayerLabels.episode(season: season.number, episode),
                                                systemImage: "play.fill")
                                        } else {
                                            Text(PlayerLabels.episode(season: season.number, episode))
                                        }
                                    }
                                }
                            }
                        }
                    } label: {
                        Label(L10n.playerEpisodes, systemImage: "list.bullet.rectangle")
                    }
                }
                if view.shuffleAvailable {
                    Toggle(isOn: Binding(get: { view.shuffle }, set: { _ in core.send(.shuffleToggled) })) {
                        Label(L10n.playerShuffle, systemImage: "shuffle")
                    }
                }
                CouchOptions(showCouch: showCouch)
            } label: {
                // sized and coloured like the system player's own buttons beside it
                Image(systemName: "slider.horizontal.3")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(.white)
                    .frame(width: 30, height: 30)
            }
            .buttonStyle(.glass)
            .buttonBorderShape(.circle)
            .simultaneousGesture(TapGesture().onEnded(onOpen))
            .accessibilityLabel(L10n.playerOptions)
            .accessibilityIdentifier("player-options")
        }

        private func choice(_ current: String, _ pick: @escaping (String) -> Void) -> Binding<String> {
            Binding(get: { current }, set: { if $0 != current { pick($0) } })
        }
    }

    /// The next episode in the last seconds, with its countdown; it starts by itself at zero.
    private struct NextUpCard: View {
        let next: NextUp
        @Environment(CoreRuntime.self) private var core

        var body: some View {
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Label(
                    L10n.playerUpNext(seconds: String(next.countdownSeconds)),
                    systemImage: next.shuffled ? "shuffle" : "forward.end"
                )
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(.white.opacity(0.75))
                .contentTransition(.numericText(countsDown: true))
                Text(PlayerLabels.nextUp(next))
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(.white)
                    .lineLimit(2)
                HStack(spacing: Tokens.Spacing.sm) {
                    Button(L10n.playerPlayNow) { core.send(.nextEpisodeRequested) }
                        .primaryAction()
                    Button(L10n.commonCancel) { core.send(.nextEpisodeCancelled) }
                        .secondaryAction()
                }
                .fixedSize()
            }
            .padding(Tokens.Spacing.lg)
            .frame(maxWidth: 320, alignment: .leading)
            .glassEffect(.regular, in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            .accessibilityElement(children: .contain)
        }
    }
#endif
