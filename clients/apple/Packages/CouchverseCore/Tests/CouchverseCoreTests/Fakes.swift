import Foundation
import Synchronization

@testable import CouchverseCore

/// Answers HTTP effects from a route table (`"GET https://..."`); anything unrouted fails as if
/// the host were unreachable, which is also how the core tries its http fallback.
final class FakeHTTP: HTTPExecuting {
    private let state = Mutex<(routes: [String: EffectOutput], requests: [HttpRequest])>(([:], []))

    var requests: [HttpRequest] { state.withLock { $0.requests } }

    func route(_ method: String, _ url: String, status: UInt16 = 200, _ body: String) {
        let output = EffectOutput.http(HttpResponse(status: status, body: body))
        state.withLock { $0.routes["\(method) \(url)"] = output }
    }

    func requests(_ method: String, _ url: String) -> [HttpRequest] {
        requests.filter { $0.method == method && $0.url == url }
    }

    func perform(_ request: HttpRequest) async -> EffectOutput {
        state.withLock { state in
            state.requests.append(request)
            return state.routes["\(request.method) \(request.url)"]
                ?? .httpFailed(HttpFailure(kind: .offline, message: "no route"))
        }
    }

    private let uploaded = Mutex<[UploadRequest]>([])

    var uploads: [UploadRequest] { uploaded.withLock { $0 } }

    /// Records the upload and answers its request from the routes.
    func upload(_ request: UploadRequest) async -> EffectOutput {
        uploaded.withLock { $0.append(request) }
        return await perform(request.request)
    }
}

/// Timers that only fire when a test says so.
@MainActor
final class ManualTimers: TimerScheduling {
    struct Started {
        let id: UInt64
        let request: TimerRequest
        let fire: @MainActor () -> Void
    }

    private(set) var running: [Started] = []
    private(set) var cancelled: [UInt64] = []

    func start(id: UInt64, request: TimerRequest, fire: @escaping @MainActor () -> Void) {
        running.append(Started(id: id, request: request, fire: fire))
    }

    func cancel(id: UInt64) {
        cancelled.append(id)
        running.removeAll { $0.id == id }
    }

    func timer(afterMs: UInt64, repeats: Bool) -> Started? {
        running.first { $0.request.afterMs == afterMs && ($0.request.repeat ?? false) == repeats }
    }

    func fire(_ id: UInt64) {
        guard let timer = running.first(where: { $0.id == id }) else { return }
        if timer.request.repeat != true {
            running.removeAll { $0.id == id }
        }
        timer.fire()
    }
}

/// The runtime's monotonic milliseconds, moved on by the test.
@MainActor
final class MillisecondClock {
    var nowMs: UInt64

    init(_ nowMs: UInt64 = 1_000) {
        self.nowMs = nowMs
    }
}

struct FailingStore: KeyValueStore {
    struct Broken: Error {}

    func read(_ key: String) throws -> String? { throw Broken() }
    func write(_ key: String, value: String) throws { throw Broken() }
    func delete(_ key: String) throws { throw Broken() }
}

/// Server payloads in the shape the API returns them.
enum Payload {
    static let serverId = "4f6c0a5e-6a43-4c0e-9d4b-2b8f8d0b7a11"

    static let server = """
        {"id":"\(serverId)","name":"Home Media","version":"1.4.0","apiLevel":1,"accent":"#3a6ea5"}
        """

    static func user(_ id: Int, _ username: String) -> String {
        """
        {"id":\(id),"username":"\(username)","displayName":"\(username.capitalized)","role":"user",\
        "bio":"","disabled":false,"createdAt":"2026-01-01T00:00:00Z","avatarId":"av-\(id)"}
        """
    }

    static func token(_ token: String, _ id: Int, _ username: String) -> String {
        #"{"deviceId":"dev-\#(id)","token":"\#(token)","user":\#(user(id, username))}"#
    }

    static let pairing = """
        {"deviceCode":"dc-secret","userCode":"WDJB-MJHT","verifyPath":"/pair?code=WDJB-MJHT",\
        "expiresIn":600,"interval":5}
        """

    static let storedServer = """
        [{"id":"\(serverId)","url":"http://tv.home","name":"Home Media","version":"1.4.0",\
        "apiLevel":1,"accent":"#3a6ea5","insecure":true}]
        """

    static let accountId = "\(serverId)/1"

    /// The persisted state of a phone signed in as the admin on `storedServer`.
    static let signedIn = [
        "servers": storedServer,
        "accounts": """
        {"accounts":[{"id":"\(accountId)","serverId":"\(serverId)","userId":1,"username":"admin",\
        "displayName":"Admin","avatarId":null,"artworkGrant":"g-1"}],"active":"\(accountId)"}
        """,
    ]

    static let directMovie = """
        {"mode":"direct","mediaFileId":"f1","grant":"gr","streamUrl":"http://tv.home/api/v1/media/gr/stream",\
        "hlsUrl":"http://tv.home/api/v1/media/gr/hls/master.m3u8","variants":[{"name":"720p","height":720}],\
        "frameUrl":"","durationSeconds":2400.0,"resumePosition":600,"allowRandomPlayback":false,\
        "display":{"title":"Glass Harbor","subtitle":"","titleId":"t1","titleSlug":"glass-harbor"},\
        "subtitles":[{"id":"s-en","lang":"en","label":"English","forced":false,\
        "url":"http://tv.home/api/v1/media/gr/subtitles/s-en.vtt"}]}
        """

    /// Routes the four loads of a freshly activated session plus the avatar grant.
    static func routeSession(_ http: FakeHTTP, base: String, user: String) {
        http.route("GET", "\(base)/api/v1/auth/me", user)
        http.route(
            "GET", "\(base)/api/v1/features",
            #"{"couchEnabled":true,"rankingsEnabled":false,"downloadsEnabled":true}"#)
        http.route("GET", "\(base)/api/v1/me/preferences", #"{"language":"cs"}"#)
        http.route("GET", "\(base)/api/v1/server", server)
        http.route("GET", "\(base)/api/v1/me/artwork-grant", #"{"grant":"g-1","expiresIn":604800}"#)
    }
}
