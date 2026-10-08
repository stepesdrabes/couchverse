import CouchverseCore
import Foundation

/// Couch view models: a host sharing its code, a follower in each of its states, a phone as a
/// remote, joins that failed and the end of a session. Avatars are identicons, the same on every
/// run.
extension Fixtures {
    static func couchMember(
        _ id: String, _ name: String, host: Bool = false, me: Bool = false, paused: Bool = false
    ) -> CouchMember {
        CouchMember(
            id: id, displayName: name, seed: name.lowercased(), host: host, anonymous: id.hasPrefix("guest"),
            paused: paused, me: me)
    }

    /// A host's couch, seen by the host.
    static let hostingMembers = [
        couchMember("p1", "Nora", host: true, me: true),
        couchMember("p2", "Otto"),
        couchMember("guest-3", "Sleepy Otter", paused: true),
    ]

    /// Štěpán's couch, seen by Nora.
    static let followingMembers = [
        couchMember("p1", "Štěpán", host: true),
        couchMember("p2", "Nora", me: true),
        couchMember("guest-3", "Sleepy Otter"),
    ]

    public static func couch(_ state: String) -> CouchView {
        let movie = PlayTarget(kind: .movie, id: "t0")
        let reactions = [
            Reaction(id: 1, participantId: "p1", emoji: "🍿"), Reaction(id: 2, participantId: "p2", emoji: "😂"),
        ]
        func live(
            _ role: CouchRole, status: CouchStatus = .open, media: PlayTarget? = movie, playing: Bool = true,
            away: Bool = false, resynced: Bool = false, reactions: [Reaction] = []
        ) -> CouchView {
            let host = role == .host
            return CouchView(
                status: status, role: role, code: "123456",
                shareUrl: host ? "http://192.168.1.5:8080/couch/123456" : nil,
                members: host ? hostingMembers : followingMembers, media: media, playing: playing,
                positionSeconds: 754, positionAtMs: 0, hostAway: away,
                waiting: role == .follower && (away || media == nil), localPaused: false, reactions: reactions,
                recentEmojis: ["🦄", "🔥"], resynced: resynced)
        }
        /// No session on this device: starting one, one that failed, or one that is over.
        func off(_ status: CouchStatus = .idle, ended: String? = nil, problem: String? = nil) -> CouchView {
            CouchView(
                status: status, members: [], playing: false, positionSeconds: 0, positionAtMs: 0, hostAway: false,
                waiting: false, localPaused: false, reactions: [], recentEmojis: [], resynced: false, ended: ended,
                problem: problem.map { Problem(code: $0, detail: "") })
        }
        return switch state {
        case "hosting": live(.host)
        case "following": live(.follower, reactions: reactions)
        case "host-paused": live(.follower, playing: false, reactions: reactions)
        case "resynced": live(.follower, resynced: true)
        case "waiting": live(.follower, media: nil, playing: false)
        case "away": live(.follower, playing: false, away: true)
        case "connecting": live(.follower, status: .connecting)
        case "remote": live(.remote)
        case "remote-paused": live(.remote, playing: false)
        case "starting": off(.connecting)
        case "start-failed": off(problem: "network")
        case "join-failed": off(problem: "no_session")
        case "not-host": off(problem: "not_host")
        case "ended": off(.ended, ended: "host_ended")
        default: off()
        }
    }

    /// A follower's player: the host's title, without timeline controls.
    public static let followerPlayer = PlayerView(
        status: .loaded, target: PlayTarget(kind: .movie, id: "t0"), title: "Glass Harbor", subtitle: "",
        titleSlug: "glass-harbor", backdrop: artwork("b", 0, accent: titles[0].accent), qualities: [], quality: "auto",
        audio: [], subtitles: [], seasons: [], shuffleAvailable: false, shuffle: false, linear: true)
}
