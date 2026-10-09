import AVFoundation
import CouchverseCore
import Observation
import UIKit
import os

private let playerLog = Logger(subsystem: "io.stepes.couchverse", category: "player")

/// Runs the core's player commands on one `AVPlayer` and reports what it does (plan 10.5): about
/// every second while playing and on every change of state, buffering, the end and failures.
/// The core decides what plays, from where and with which tracks; this only executes it.
@Observable
public final class PlayerController: PlayerExecuting {
    @ObservationIgnored public var events: ((Event) -> Void)?
    public let player = AVPlayer()

    /// The sidecar subtitle line to draw over the picture, if any.
    private(set) var caption: String?
    /// The system's own menus list the item's audio renditions or subtitles; when they do not,
    /// the screen offers the core's tracks itself. The core may also offer another language's
    /// file, which no menu of the item's lists.
    var nativeAudio: Bool { nativeAudioOptions > 1 && nativeAudioOptions >= audioTracks.count }
    private(set) var nativeSubtitles = false
    /// The item's audio renditions, which the system's menu lists when there are two or more.
    private(set) var nativeAudioOptions = 0
    /// A couch follower's player: no seeking or pausing.
    private(set) var linear = false
    /// Playing or waiting to, rather than paused.
    private(set) var playing = false
    /// The core's audio tracks and the chosen one, to name a language picked in the system's
    /// menu.
    @ObservationIgnored var audioTracks: [TrackOption] = []
    @ObservationIgnored var audioSelected: String?

    @ObservationIgnored private var load: PlayerLoad?
    @ObservationIgnored private var subtitle: String?
    @ObservationIgnored private var audioLang: String?
    /// The rendition's place among the stream's audio options, when the core named one.
    @ObservationIgnored private var audioIndex: Int?
    @ObservationIgnored private var cues: [WebVTTCue] = []
    @ObservationIgnored private var cueTask: Task<Void, Never>?
    @ObservationIgnored private var captionObserver: Any?
    @ObservationIgnored private var reportObserver: Any?
    @ObservationIgnored private var statusObservation: NSKeyValueObservation?
    @ObservationIgnored private var itemObservation: NSKeyValueObservation?
    @ObservationIgnored private var itemTokens: [NSObjectProtocol] = []
    /// Set from a load until the item is ready and at its start position: the first frames sit
    /// at zero, which the core would take for a seek and save as progress.
    @ObservationIgnored private var settling = false
    @ObservationIgnored private var localPause = LocalPause()
    @ObservationIgnored private var generation = 0
    @ObservationIgnored private let downloads: URL

    /// Where finished downloads live; a `download` source names a file in it.
    public static var downloadsDirectory: URL {
        URL.applicationSupportDirectory.appending(path: "Downloads", directoryHint: .isDirectory)
    }

    public init(downloads: URL = PlayerController.downloadsDirectory) {
        self.downloads = downloads
        player.allowsExternalPlayback = true
        statusObservation = player.observe(\.timeControlStatus) { @Sendable [weak self] _, _ in
            Task { @MainActor in
                guard let self else { return }
                let playing = self.player.timeControlStatus != .paused
                if playing != self.playing {
                    self.playing = playing
                }
                self.followerPaused(playing: playing)
                self.report()
            }
        }
        reportObserver = player.addPeriodicTimeObserver(
            forInterval: CMTime(seconds: 1, preferredTimescale: 600), queue: .main
        ) { [weak self] _ in
            MainActor.assumeIsolated { self?.report() }
        }
    }

    public func execute(_ command: PlayerCommand) {
        switch command {
        case .load(let load):
            start(load)
        case .play:
            localPause.commanded(playing: true)
            player.play()
        case .pause:
            localPause.commanded(playing: false)
            player.pause()
        case .seek(let seek):
            let time = CMTime(seconds: seek.seconds, preferredTimescale: 600)
            player.seek(to: time, toleranceBefore: .zero, toleranceAfter: .zero) { [weak self] _ in
                Task { @MainActor in self?.report() }
            }
        case .selectAudio(let rendition):
            audioLang = rendition.lang
            audioIndex = rendition.index.map(Int.init)
            Task { await applyAudio() }
        case .selectSubtitles(let selection):
            subtitle = selection.id
            Task { await applySubtitles() }
        case .stop:
            stop()
        }
    }

    // MARK: - Loading

    private func start(_ load: PlayerLoad) {
        clearItem()
        generation += 1
        let generation = generation
        self.load = load
        subtitle = load.subtitle
        audioLang = load.audioLang
        audioIndex = load.audioIndex.map(Int.init)
        linear = load.linear
        localPause = LocalPause()
        settling = true
        guard let url = url(for: load) else {
            settling = false
            report(failed: "unreadable address")
            return
        }
        let asset = AVURLAsset(url: url)
        let item = AVPlayerItem(asset: asset)
        if let height = load.maxHeight {
            item.preferredMaximumResolution = CGSize(
                width: (Double(height) * 16 / 9).rounded(.up), height: Double(height))
        }
        item.externalMetadata = Self.metadata(load.nowPlaying, artwork: nil)
        observe(item)
        player.replaceCurrentItem(with: item)
        Task { await prepare(item, load: load, generation: generation) }
        Task { await attachArtwork(to: item, load.nowPlaying, generation: generation) }
    }

    private func url(for load: PlayerLoad) -> URL? {
        switch load.source {
        case .download: downloads.appending(path: load.url, directoryHint: .notDirectory)
        case .file, .hls: URL(string: load.url)
        }
    }

    /// Waits for the item, moves it to the start position, applies the tracks, then plays.
    private func prepare(_ item: AVPlayerItem, load: PlayerLoad, generation: Int) async {
        let ready = await Self.ready(item)
        guard generation == self.generation else { return }
        guard ready else {
            settling = false
            report(failed: item.error?.localizedDescription ?? "the item failed to load")
            return
        }
        if load.startSeconds > 0 {
            let start = CMTime(seconds: load.startSeconds, preferredTimescale: 600)
            await player.seek(to: start, toleranceBefore: .zero, toleranceAfter: .zero)
        }
        await applyAudio()
        await applySubtitles()
        await describeOptions(item, load: load)
        guard generation == self.generation else { return }
        settling = false
        localPause.commanded(playing: load.autoplay)
        if load.autoplay {
            player.play()
        }
        report()
    }

    /// Resolves once the item is ready to play (true) or has failed (false).
    private static func ready(_ item: AVPlayerItem) async -> Bool {
        if item.status != .unknown { return item.status == .readyToPlay }
        var observation: NSKeyValueObservation?
        defer { observation?.invalidate() }
        return await withCheckedContinuation { continuation in
            let once = Once()
            observation = item.observe(\.status, options: [.initial, .new]) { @Sendable item, _ in
                let status = item.status
                guard status != .unknown, once.claim() else { return }
                continuation.resume(returning: status == .readyToPlay)
            }
        }
    }

    private func observe(_ item: AVPlayerItem) {
        let observed = ObjectIdentifier(item)
        itemObservation = item.observe(\.status) { @Sendable [weak self] item, _ in
            guard item.status == .failed else { return }
            let reason = item.error?.localizedDescription ?? "the item failed"
            Task { @MainActor in
                guard let self, let current = self.player.currentItem, ObjectIdentifier(current) == observed else {
                    return
                }
                self.settling = false
                self.report(failed: reason)
            }
        }
        let center = NotificationCenter.default
        itemTokens = [
            center.addObserver(forName: AVPlayerItem.didPlayToEndTimeNotification, object: item, queue: .main) {
                [weak self] _ in
                MainActor.assumeIsolated { self?.report(ended: true) }
            },
            center.addObserver(forName: AVPlayerItem.failedToPlayToEndTimeNotification, object: item, queue: .main) {
                [weak self] note in
                let error = note.userInfo?[AVPlayerItemFailedToPlayToEndTimeErrorKey] as? Error
                let reason = error?.localizedDescription ?? "playback stopped"
                MainActor.assumeIsolated { self?.report(failed: reason) }
            },
            center.addObserver(forName: AVPlayerItem.mediaSelectionDidChangeNotification, object: item, queue: .main) {
                [weak self] _ in
                MainActor.assumeIsolated { self?.nativeSelectionChanged() }
            },
        ]
    }

    private func stop() {
        player.pause()
        clearItem()
        player.replaceCurrentItem(with: nil)
        load = nil
        linear = false
    }

    private func clearItem() {
        generation += 1
        itemObservation?.invalidate()
        itemObservation = nil
        itemTokens.forEach(NotificationCenter.default.removeObserver)
        itemTokens = []
        clearCaptions()
        nativeAudioOptions = 0
        nativeSubtitles = false
    }

    // MARK: - Reports

    /// A couch follower's player started or stopped without the core asking: the viewer pressed
    /// play or pause in the system's controls (or a call paused it). The core keeps them paused for
    /// themselves, rather than snapping them back to the host at its next update.
    private func followerPaused(playing: Bool) {
        guard linear, !settling, let item = player.currentItem, item.status == .readyToPlay else { return }
        // the end of the item pauses it too; the host's next title follows
        let end = item.duration.isNumeric && item.currentTime().seconds >= item.duration.seconds - 1
        guard !end, let paused = localPause.observed(playing: playing) else { return }
        events?(.couchLocalPauseChanged(CouchPause(paused: paused)))
    }

    private func report(ended: Bool = false, failed: String? = nil) {
        guard let load, let item = player.currentItem else { return }
        if settling && failed == nil { return }
        let status = player.timeControlStatus
        let position = player.currentTime().seconds
        let duration = item.duration.isNumeric ? item.duration.seconds : load.nowPlaying.durationSeconds
        events?(
            .playerReported(
                PlayerReport(
                    positionSeconds: position.isFinite ? max(position, 0) : 0,
                    durationSeconds: duration.isFinite ? duration : 0,
                    playing: !ended && failed == nil && status != .paused,
                    buffering: status == .waitingToPlayAtSpecifiedRate,
                    ended: ended ? true : nil, failed: failed)))
    }

    // MARK: - Tracks

    /// The rendition at the core's index when there is one in that language (two can share a
    /// language: a film's own track and a commentary), else the first in the language.
    private func applyAudio() async {
        guard let item = player.currentItem, let lang = audioLang,
            let group = try? await item.asset.loadMediaSelectionGroup(for: .audible)
        else { return }
        let indexed = audioIndex.flatMap { group.options.indices.contains($0) ? group.options[$0] : nil }
        guard
            let option = indexed.flatMap({ Self.language(of: $0) == Self.normalized(lang) ? $0 : nil })
                ?? Self.option(in: group, lang: lang, forced: false)
        else { return }
        if item.currentMediaSelection.selectedMediaOption(in: group) != option {
            item.select(option, in: group)
        }
    }

    /// A sidecar WebVTT file is drawn by the screen for a progressive source; a track inside the
    /// media (an HLS rendition, a download's own subtitles) is selected by language, falling back
    /// to the sidecar file when the media has no such track.
    private func applySubtitles() async {
        guard let item = player.currentItem, let load else { return }
        clearCaptions()
        let group = try? await item.asset.loadMediaSelectionGroup(for: .legible)
        guard let track = load.subtitles.first(where: { $0.id == subtitle }) else {
            if let group, group.allowsEmptySelection {
                item.select(nil, in: group)
            }
            return
        }
        if load.source != .hls, let url = track.url.flatMap(URL.init(string:)) {
            if let group, group.allowsEmptySelection {
                item.select(nil, in: group)
            }
            showSidecar(url)
        } else if let group, let option = Self.option(in: group, lang: track.lang, forced: track.forced) {
            item.select(option, in: group)
        } else if let url = track.url.flatMap(URL.init(string:)) {
            showSidecar(url)
        }
    }

    private func describeOptions(_ item: AVPlayerItem, load: PlayerLoad) async {
        let audible = try? await item.asset.loadMediaSelectionGroup(for: .audible)
        let legible = try? await item.asset.loadMediaSelectionGroup(for: .legible)
        nativeAudioOptions = audible?.options.count ?? 0
        let sidecars = load.source != .hls && load.subtitles.contains { $0.url != nil }
        nativeSubtitles = !(legible?.options.isEmpty ?? true) && !sidecars
    }

    /// A language picked in the system's own menu becomes the core's choice, so it is remembered
    /// like one picked anywhere else.
    private func nativeSelectionChanged() {
        guard !settling, let item = player.currentItem, let load else { return }
        Task {
            if nativeSubtitles, let group = try? await item.asset.loadMediaSelectionGroup(for: .legible) {
                let option = item.currentMediaSelection.selectedMediaOption(in: group)
                let id = option.flatMap { option in
                    load.subtitles.first {
                        Self.language(of: option) == Self.normalized($0.lang)
                            && option.hasMediaCharacteristic(.containsOnlyForcedSubtitles) == $0.forced
                    }?.id
                }
                if id != subtitle {
                    subtitle = id
                    events?(.subtitlesChosen(TrackChoice(id: id)))
                }
            }
            if nativeAudioOptions > 1, let group = try? await item.asset.loadMediaSelectionGroup(for: .audible),
                let option = item.currentMediaSelection.selectedMediaOption(in: group),
                let index = group.options.firstIndex(of: option), index != audioIndex
            {
                let track =
                    itemTrack(at: index)
                    ?? audioTracks.first { Self.normalized($0.lang) == Self.language(of: option) }
                if let track {
                    audioLang = track.lang
                    audioIndex = index
                    events?(.audioChosen(TrackChoice(id: track.id)))
                }
            }
        }
    }

    /// The core's track for the item's rendition at `index`. The core lists a file's renditions
    /// together and in order, so the item's first sits as many tracks before the chosen one as
    /// its place says; nil when the chosen one is a whole file, whose language names the track.
    private func itemTrack(at index: Int) -> TrackOption? {
        guard let place = audioIndex, let chosen = audioTracks.firstIndex(where: { $0.id == audioSelected }) else {
            return nil
        }
        let at = chosen - place + index
        return audioTracks.indices.contains(at) ? audioTracks[at] : nil
    }

    static func option(in group: AVMediaSelectionGroup, lang: String, forced: Bool) -> AVMediaSelectionOption? {
        let wanted = normalized(lang)
        let matching = group.options.filter { language(of: $0) == wanted }
        let isForced = { (option: AVMediaSelectionOption) in option.hasMediaCharacteristic(.containsOnlyForcedSubtitles)
        }
        return matching.first { isForced($0) == forced } ?? matching.first
    }

    static func language(of option: AVMediaSelectionOption) -> String? {
        (option.locale?.language.languageCode?.identifier ?? option.extendedLanguageTag).map(normalized)
    }

    /// ISO 639-1 where there is one: the core says `cs`, a stream may say `ces` or `cs-CZ`.
    static func normalized(_ tag: String) -> String {
        let base = String(tag.split(separator: "-").first ?? Substring(tag)).lowercased()
        let terminology = bibliographic[base] ?? base
        return Locale.LanguageCode(terminology).identifier(.alpha2) ?? terminology
    }

    /// ISO 639-2/B codes some muxers write (`cze`), which Foundation only knows in their /T form.
    private static let bibliographic = [
        "alb": "sqi", "arm": "hye", "baq": "eus", "bur": "mya", "chi": "zho", "cze": "ces", "dut": "nld",
        "fre": "fra", "geo": "kat", "ger": "deu", "gre": "ell", "ice": "isl", "mac": "mkd", "mao": "mri",
        "may": "msa", "per": "fas", "rum": "ron", "slo": "slk", "tib": "bod", "wel": "cym",
    ]

    // MARK: - Sidecar subtitles

    private func showSidecar(_ url: URL) {
        let generation = generation
        cueTask = Task { [weak self] in
            guard let (data, _) = try? await URLSession.shared.data(from: url) else {
                playerLog.error("subtitles did not load")
                return
            }
            guard let self, generation == self.generation, !Task.isCancelled else { return }
            cues = WebVTT.parse(String(decoding: data, as: UTF8.self))
            captionObserver = player.addPeriodicTimeObserver(
                forInterval: CMTime(seconds: 0.2, preferredTimescale: 600), queue: .main
            ) { [weak self] time in
                MainActor.assumeIsolated {
                    guard let self else { return }
                    let text = WebVTT.text(at: time.seconds, in: self.cues)
                    if text != self.caption {
                        self.caption = text
                    }
                }
            }
        }
    }

    private func clearCaptions() {
        cueTask?.cancel()
        cueTask = nil
        if let captionObserver {
            player.removeTimeObserver(captionObserver)
        }
        captionObserver = nil
        cues = []
        caption = nil
    }

    // MARK: - Now Playing

    /// The title, episode and backdrop for the system's Now Playing, lock screen and info panel.
    static func metadata(_ info: NowPlaying, artwork: Data?) -> [AVMetadataItem] {
        var items = [metadataItem(.commonIdentifierTitle, info.title as NSString)]
        if let subtitle = info.subtitle {
            items.append(metadataItem(.iTunesMetadataTrackSubTitle, subtitle as NSString))
        }
        if let artwork {
            items.append(metadataItem(.commonIdentifierArtwork, artwork as NSData))
        }
        return items
    }

    private static func metadataItem(
        _ identifier: AVMetadataIdentifier, _ value: NSCopying & NSObjectProtocol
    )
        -> AVMetadataItem
    {
        let item = AVMutableMetadataItem()
        item.identifier = identifier
        item.value = value
        item.extendedLanguageTag = "und"
        return item
    }

    private func attachArtwork(to item: AVPlayerItem, _ info: NowPlaying, generation: Int) async {
        guard let url = info.artwork.flatMap(URL.init(string:)),
            let (data, _) = try? await URLSession.shared.data(from: url),
            let jpeg = UIImage(data: data)?.jpegData(compressionQuality: 0.85),
            generation == self.generation
        else { return }
        item.externalMetadata = Self.metadata(info, artwork: jpeg)
    }
}

/// Lets exactly one of several callbacks through.
nonisolated private final class Once: Sendable {
    private let lock = OSAllocatedUnfairLock(initialState: false)

    func claim() -> Bool {
        lock.withLock { claimed in
            defer { claimed = true }
            return !claimed
        }
    }
}

/// Tells a pause or play the viewer made from one the core asked for: whatever differs from the
/// core's last command was the viewer's own.
struct LocalPause {
    /// What the core last asked for: playing, or paused; nil before it asked.
    private(set) var expected: Bool?

    mutating func commanded(playing: Bool) {
        expected = playing
    }

    /// The player started (true) or stopped; returns the viewer's own pause (true) or resume
    /// (false), or nil when it is what the core asked for.
    mutating func observed(playing: Bool) -> Bool? {
        guard let expected, expected != playing else { return nil }
        self.expected = playing
        return !playing
    }
}
