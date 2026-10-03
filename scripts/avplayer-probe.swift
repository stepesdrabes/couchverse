// Loads a media URL the way the Apple apps will (AVURLAsset, no cookies or
// headers), plays a few seconds of it and fails unless AVFoundation decodes it.
// For HLS it also loads every variant and rendition playlist on its own, lists
// the audio and subtitle media selection groups, switches to every audio and
// subtitle option while playing, and reports whether an I-frame playlist made
// trick play (scrubbing thumbnails) available.
//
// usage: swift scripts/avplayer-probe.swift <url> [--duration <seconds>] [--play <seconds>]
//        [--audio <n>] [--subtitles <n>] [--iframes]
// The --audio/--subtitles/--iframes flags turn expectations into failures: at least
// n audible (legible) options, and fast forward through an I-frame playlist.
import AVFoundation
import CoreMedia
import Foundation

struct Options {
    var url: URL
    var duration: Double?
    var play = 3.0
    var audio = 0
    var subtitles = 0
    var iframes = false
}

func parseOptions() -> Options {
    var args = Array(CommandLine.arguments.dropFirst())
    guard let first = args.first, let url = URL(string: first) else {
        FileHandle.standardError.write(Data("usage: avplayer-probe <url> [--duration s] [--play s] [--audio n] [--subtitles n] [--iframes]\n".utf8))
        exit(2)
    }
    args.removeFirst()
    var opts = Options(url: url)
    while !args.isEmpty {
        let flag = args.removeFirst()
        switch flag {
        case "--iframes": opts.iframes = true
        case "--duration", "--play", "--audio", "--subtitles":
            guard let value = args.first.flatMap(Double.init) else {
                fail("\(flag) needs a number")
            }
            args.removeFirst()
            switch flag {
            case "--duration": opts.duration = value
            case "--play": opts.play = value
            case "--audio": opts.audio = Int(value)
            default: opts.subtitles = Int(value)
            }
        default: fail("unknown flag \(flag)")
        }
    }
    return opts
}

var failures: [String] = []

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data("FAIL \(message)\n".utf8))
    exit(1)
}

func check(_ ok: Bool, _ message: String) {
    if !ok {
        failures.append(message)
        FileHandle.standardError.write(Data("FAIL \(message)\n".utf8))
    }
}

func fourCC(_ code: FourCharCode) -> String {
    let bytes = [24, 16, 8, 0].map { UInt8((code >> $0) & 0xff) }
    return String(bytes: bytes, encoding: .ascii) ?? "\(code)"
}

/// The URIs a multivariant playlist references (variants, renditions, I-frame streams),
/// resolved against the playlist URL.
func referencedPlaylists(master: URL) async throws -> [(kind: String, url: URL)] {
    let (data, _) = try await URLSession.shared.data(from: master)
    guard let text = String(data: data, encoding: .utf8), text.contains("#EXT-X-STREAM-INF") else {
        return []
    }
    var out: [(String, URL)] = []
    var expectVariant = false
    for line in text.split(whereSeparator: \.isNewline).map(String.init) {
        if line.hasPrefix("#EXT-X-STREAM-INF") {
            expectVariant = true
        } else if line.hasPrefix("#EXT-X-MEDIA") || line.hasPrefix("#EXT-X-I-FRAME-STREAM-INF") {
            if let range = line.range(of: "URI=\""), let end = line[range.upperBound...].firstIndex(of: "\"") {
                let uri = String(line[range.upperBound..<end])
                let kind = line.hasPrefix("#EXT-X-MEDIA") ? (line.contains("TYPE=AUDIO") ? "audio" : "subtitles") : "iframes"
                if let url = URL(string: uri, relativeTo: master) { out.append((kind, url.absoluteURL)) }
            }
        } else if expectVariant, !line.hasPrefix("#") {
            if let url = URL(string: line, relativeTo: master) { out.append(("variant", url.absoluteURL)) }
            expectVariant = false
        }
    }
    return out
}

func describeVariants(_ asset: AVURLAsset) async throws {
    for variant in try await asset.load(.variants) {
        var parts = ["peak=\(Int(variant.peakBitRate ?? 0))"]
        if let avg = variant.averageBitRate { parts.append("avg=\(Int(avg))") }
        if let video = variant.videoAttributes {
            let codecs = video.codecTypes.map(fourCC).joined(separator: "+")
            parts.append("video=\(codecs) \(Int(video.presentationSize.width))x\(Int(video.presentationSize.height))")
            parts.append("range=\(video.videoRange.rawValue)")
            if let fps = video.nominalFrameRate { parts.append(String(format: "fps=%.3f", fps)) }
        }
        if let audio = variant.audioAttributes {
            parts.append("audio=\(audio.formatIDs.map(fourCC).joined(separator: "+"))")
        }
        print("  variant \(parts.joined(separator: " "))")
    }
}

func selectionOptions(_ asset: AVURLAsset, _ characteristic: AVMediaCharacteristic) async throws -> AVMediaSelectionGroup? {
    guard let group = try await asset.loadMediaSelectionGroup(for: characteristic) else { return nil }
    for option in group.options {
        let lang = option.extendedLanguageTag ?? option.locale?.identifier ?? "und"
        let forced = option.hasMediaCharacteristic(.containsOnlyForcedSubtitles) ? " forced" : ""
        let def = group.defaultOption == option ? " default" : ""
        print("  \(characteristic == .audible ? "audio" : "subtitles") \(lang) \"\(option.displayName)\"\(forced)\(def)")
    }
    return group
}

struct PlayResult {
    var ok: Bool
    var trickPlay: Bool
}

@MainActor
func play(_ asset: AVURLAsset, seconds: Double, audible: AVMediaSelectionGroup?, legible: AVMediaSelectionGroup?, expectVideo: Bool) async -> PlayResult {
    let item = AVPlayerItem(asset: asset)
    let output = AVPlayerItemVideoOutput(pixelBufferAttributes: nil)
    item.add(output)
    let player = AVPlayer(playerItem: item)
    player.isMuted = true
    player.play()

    var frames = 0
    var lastFrame = CMTime.invalid
    let start = Date()
    var switched = 0
    // one tick per 50 ms; every audible and legible option gets a turn halfway through
    let switches: [(AVMediaSelectionGroup, AVMediaSelectionOption?)] =
        (audible.map { g in g.options.map { (g, Optional($0)) } } ?? []) +
        (legible.map { g in g.options.map { (g, Optional($0)) } + [(g, nil)] } ?? [])
    while Date().timeIntervalSince(start) < seconds + 10 {
        try? await Task.sleep(for: .milliseconds(50))
        if item.status == .failed { break }
        let now = item.currentTime()
        if output.hasNewPixelBuffer(forItemTime: now), output.copyPixelBuffer(forItemTime: now, itemTimeForDisplay: nil) != nil, now != lastFrame {
            frames += 1
            lastFrame = now
        }
        if now.seconds >= seconds { break }
        if item.status == .readyToPlay, now.seconds > seconds / 2, switched < switches.count {
            let (group, option) = switches[switched]
            item.select(option, in: group)
            switched += 1
        }
    }
    let reached = item.currentTime().seconds
    var ok = item.status == .readyToPlay && reached >= min(seconds, 1)
    print(String(format: "  played %.2fs, %d frames decoded, status=%d", reached, frames, item.status.rawValue))
    if let error = item.error {
        print("  error: \(error)")
        ok = false
    }
    for event in item.errorLog()?.events ?? [] {
        print("  errorLog: \(event.errorStatusCode) \(event.errorDomain) \(event.errorComment ?? "")")
    }
    if expectVideo, frames == 0 {
        print("  no video frame was decoded")
        ok = false
    }
    print("  trick play: fastForward=\(item.canPlayFastForward) fastReverse=\(item.canPlayFastReverse)")
    player.pause()
    return PlayResult(ok: ok, trickPlay: item.canPlayFastForward)
}

let opts = parseOptions()
let asset = AVURLAsset(url: opts.url)
do {
    let (playable, duration) = try await asset.load(.isPlayable, .duration)
    print("\(opts.url.lastPathComponent): playable=\(playable) duration=\(String(format: "%.2f", duration.seconds))")
    check(playable, "not playable")
    if let expected = opts.duration {
        check(abs(duration.seconds - expected) <= 1, "duration \(duration.seconds) differs from \(expected)")
    }

    let isHLS = opts.url.path.hasSuffix(".m3u8")
    var expectVideo = true
    if isHLS {
        try await describeVariants(asset)
        for (kind, url) in try await referencedPlaylists(master: opts.url) {
            let sub = AVURLAsset(url: url)
            let (ok, d) = try await sub.load(.isPlayable, .duration)
            // subtitle and I-frame playlists are not playable on their own
            let label = "\(kind) \(url.pathComponents.suffix(2).joined(separator: "/"))"
            print("  \(label): playable=\(ok) duration=\(String(format: "%.2f", d.seconds))")
            if kind == "variant" || kind == "audio" {
                check(ok, "\(label) not playable on its own")
                check(abs(d.seconds - duration.seconds) <= 1, "\(label) duration \(d.seconds) differs from \(duration.seconds)")
            }
        }
    } else {
        let tracks = try await asset.load(.tracks)
        expectVideo = tracks.contains { $0.mediaType == .video }
        print("  tracks: \(tracks.map(\.mediaType.rawValue).sorted().joined(separator: ","))")
    }

    let audible = try await selectionOptions(asset, .audible)
    let legible = try await selectionOptions(asset, .legible)
    check((audible?.options.count ?? 0) >= opts.audio, "expected at least \(opts.audio) audio options")
    check((legible?.options.count ?? 0) >= opts.subtitles, "expected at least \(opts.subtitles) subtitle options")

    let played = await play(asset, seconds: opts.play, audible: audible, legible: legible, expectVideo: expectVideo)
    check(played.ok, "playback failed")
    if opts.iframes {
        check(played.trickPlay, "no trick play: the I-frame playlist was not usable")
    }
} catch {
    fail("not playable: \(error)")
}
if !failures.isEmpty {
    exit(1)
}
print("ok")
