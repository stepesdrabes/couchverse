import Foundation

/// One timed line of a WebVTT file.
struct WebVTTCue: Equatable, Sendable {
    let start: Double
    let end: Double
    let text: String
}

/// The subset of WebVTT the server writes: cues with optional identifiers and settings, notes,
/// and inline tags, which are dropped (styling is the shell's). AVPlayer cannot attach a sidecar
/// file to progressive media, so the shell parses and draws these itself.
enum WebVTT {
    static func parse(_ source: String) -> [WebVTTCue] {
        let normalized = source.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        var cues: [WebVTTCue] = []
        for block in normalized.components(separatedBy: "\n\n") {
            let lines = block.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
            guard let timing = lines.firstIndex(where: { $0.contains("-->") }) else { continue }
            let first = lines.first?.trimmingCharacters(in: .whitespaces) ?? ""
            if first.hasPrefix("NOTE") || first.hasPrefix("STYLE") || first.hasPrefix("REGION") {
                continue
            }
            let parts = lines[timing].components(separatedBy: "-->")
            guard parts.count == 2, let start = timestamp(parts[0]),
                let end = timestamp(parts[1].trimmingCharacters(in: .whitespaces).components(separatedBy: " ")[0])
            else { continue }
            let text = lines[(timing + 1)...]
                .map(stripTags)
                .joined(separator: "\n")
                .trimmingCharacters(in: .whitespacesAndNewlines)
            if !text.isEmpty && end > start {
                cues.append(WebVTTCue(start: start, end: end, text: text))
            }
        }
        return cues.sorted { $0.start < $1.start }
    }

    /// The text shown at `time`: every cue covering it, earliest first.
    static func text(at time: Double, in cues: [WebVTTCue]) -> String? {
        // cues are sorted by start; only those starting before `time` can cover it
        var low = 0
        var high = cues.count
        while low < high {
            let mid = (low + high) / 2
            if cues[mid].start <= time { low = mid + 1 } else { high = mid }
        }
        let shown = cues[..<low].filter { $0.end > time }.map(\.text)
        return shown.isEmpty ? nil : shown.joined(separator: "\n")
    }

    /// `hh:mm:ss.ttt` or `mm:ss.ttt`.
    static func timestamp(_ raw: String) -> Double? {
        let fields = raw.trimmingCharacters(in: .whitespaces).split(separator: ":")
        guard (2...3).contains(fields.count) else { return nil }
        var seconds = 0.0
        for field in fields.dropLast() {
            guard let value = Int(field) else { return nil }
            seconds = seconds * 60 + Double(value)
        }
        guard let last = Double(fields.last!.replacingOccurrences(of: ",", with: ".")) else { return nil }
        return seconds * 60 + last
    }

    private static func stripTags(_ line: String) -> String {
        var out = ""
        var inTag = false
        for character in line {
            switch character {
            case "<": inTag = true
            case ">" where inTag: inTag = false
            default: if !inTag { out.append(character) }
            }
        }
        return
            out
            .replacingOccurrences(of: "&lt;", with: "<")
            .replacingOccurrences(of: "&gt;", with: ">")
            .replacingOccurrences(of: "&nbsp;", with: "\u{00A0}")
            .replacingOccurrences(of: "&amp;", with: "&")
    }
}
