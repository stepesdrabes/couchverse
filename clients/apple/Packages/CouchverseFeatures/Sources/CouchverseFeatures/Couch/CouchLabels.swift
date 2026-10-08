import CouchverseCore
import CouchverseDesign
import Foundation

extension CouchView {
    /// This device is on a couch, or getting there.
    var isLive: Bool {
        role != nil && (status == .connecting || status == .open || status == .reconnecting)
    }

    var host: CouchMember? {
        members.first { $0.host }
    }
}

// The core hands over a couch's state as codes and flags, never UI words; these are its words in
// the display language, as on the web and Android.
enum CouchLabels {
    /// The one line that says what the session is doing, when it needs saying.
    static func status(_ view: CouchView) -> String? {
        if view.status == .connecting {
            return L10n.couchConnecting
        }
        if view.status == .reconnecting {
            return L10n.couchReconnecting
        }
        if view.status == .ended {
            return view.ended == "host_ended" ? L10n.couchSessionEndedHost : L10n.couchSessionEndedTitle
        }
        guard view.role == .follower else { return nil }
        if view.hostAway {
            return L10n.couchHostAway
        }
        if view.waiting && view.media == nil {
            return view.host.map { L10n.couchHostChoosing(name: $0.displayName) } ?? L10n.couchHostChoosingGeneric
        }
        if view.resynced {
            return L10n.couchResynced
        }
        if !view.playing && !view.localPaused {
            return L10n.couchHostPaused
        }
        return nil
    }

    /// "123 456": six digits read out in two halves.
    static func spaced(_ code: String) -> String {
        code.count == 6 ? "\(code.prefix(3)) \(code.suffix(3))" : code
    }

    /// A member's badges: the host (and whether they stepped away), this device, a paused member.
    static func badges(_ member: CouchMember, hostAway: Bool) -> String {
        var badges: [String] = []
        if member.host {
            badges.append(hostAway ? L10n.couchHostAway : L10n.couchHostBadge)
        }
        if member.me {
            badges.append(L10n.couchYouBadge)
        }
        if member.paused {
            badges.append(L10n.couchMemberPaused)
        }
        return badges.joined(separator: " \u{00B7} ")
    }

    /// Where the host is now: its last position, moved on by the time since while it plays.
    static func position(_ view: CouchView, now: UInt64) -> Double {
        guard view.playing, now > view.positionAtMs else { return view.positionSeconds }
        return view.positionSeconds + Double(now - view.positionAtMs) / 1000
    }
}

/// A couch code as typed: digits only, at most six.
enum CouchCodeInput {
    static func format(_ typed: String) -> String {
        String(typed.filter { $0.isASCII && $0.isNumber }.prefix(6))
    }

    static func isComplete(_ code: String) -> Bool {
        code.count == 6 && format(code) == code
    }
}

enum CouchLink {
    /// The six-digit code of a couch link: `couchverse://couch/123456`, or the join page a host's
    /// QR code shows (`https://media.example.com/couch/123456`).
    static func code(_ string: String) -> String? {
        guard let url = URL(string: string.trimmingCharacters(in: .whitespacesAndNewlines)) else { return nil }
        let path = url.pathComponents.filter { $0 != "/" }
        let parts: [String]
        switch url.scheme?.lowercased() {
        case "couchverse": parts = [url.host() ?? ""] + path
        case "http", "https": parts = path
        default: return nil
        }
        guard parts.count == 2, parts[0].lowercased() == "couch", CouchCodeInput.isComplete(parts[1]) else {
            return nil
        }
        return parts[1]
    }
}

enum Reactions {
    /// The reactions offered first, the same set as Android's.
    static let quick = ["❤️", "😂", "😮", "😢", "👏", "🔥", "🍿", "👍", "🎉", "😱", "🤔", "😴"]

    /// What the viewer sent lately first, then the quick ones not among them.
    static func choices(recent: [String]) -> [String] {
        var seen = Set<String>()
        return (recent + quick).filter { seen.insert($0).inserted }
    }
}
