// Loads a media URL the way the Apple apps will (AVURLAsset, no cookies or
// headers) and fails unless AVFoundation finds it playable.
// usage: swift scripts/avplayer-probe.swift <url> [expected-seconds]
import AVFoundation
import Foundation

let args = CommandLine.arguments
guard args.count >= 2, let url = URL(string: args[1]) else {
    FileHandle.standardError.write(Data("usage: avplayer-probe <url> [expected-seconds]\n".utf8))
    exit(2)
}

let asset = AVURLAsset(url: url)
do {
    let (playable, duration, tracks) = try await asset.load(.isPlayable, .duration, .tracks)
    let kinds = tracks.map(\.mediaType.rawValue).sorted().joined(separator: ",")
    print("playable=\(playable) duration=\(String(format: "%.2f", duration.seconds)) tracks=\(kinds)")
    if !playable {
        exit(1)
    }
    if args.count >= 3, let expected = Double(args[2]), abs(duration.seconds - expected) > 0.5 {
        FileHandle.standardError.write(Data("duration \(duration.seconds) differs from \(expected)\n".utf8))
        exit(1)
    }
} catch {
    FileHandle.standardError.write(Data("not playable: \(error)\n".utf8))
    exit(1)
}
