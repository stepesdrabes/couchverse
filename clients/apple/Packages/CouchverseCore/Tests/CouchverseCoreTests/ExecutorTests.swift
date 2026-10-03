import Foundation
import Synchronization
import Testing

@testable import CouchverseCore

/// Serves canned answers to a URLSession, keyed by URL.
final class StubProtocol: URLProtocol {
    enum Answer {
        case response(status: Int, body: String)
        case failure(URLError.Code)
    }

    nonisolated(unsafe) static var answers: [String: Answer] = [:]
    nonisolated(unsafe) static var seen: [URLRequest] = []
    static let lock = NSLock()

    static func session() -> URLSession {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubProtocol.self]
        return URLSession(configuration: configuration)
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let answer = Self.lock.withLock {
            var recorded = request
            // URLSession hands bodies to protocols as a stream
            if let stream = request.httpBodyStream {
                stream.open()
                var data = Data()
                var buffer = [UInt8](repeating: 0, count: 1024)
                while stream.hasBytesAvailable {
                    let read = stream.read(&buffer, maxLength: buffer.count)
                    guard read > 0 else { break }
                    data.append(buffer, count: read)
                }
                stream.close()
                recorded.httpBody = data
            }
            Self.seen.append(recorded)
            return Self.answers[request.url?.absoluteString ?? ""]
        }
        switch answer {
        case .response(let status, let body):
            let response = HTTPURLResponse(
                url: request.url!, statusCode: status, httpVersion: "HTTP/1.1", headerFields: nil)!
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: Data(body.utf8))
            client?.urlProtocolDidFinishLoading(self)
        case .failure(let code):
            client?.urlProtocol(self, didFailWithError: URLError(code))
        case nil:
            client?.urlProtocol(self, didFailWithError: URLError(.cannotConnectToHost))
        }
    }

    override func stopLoading() {}
}

@Suite(.serialized)
struct HTTPExecutorTests {
    let executor = HTTPExecutor(session: StubProtocol.session())

    @Test func answersCarryTheStatusAndBody() async {
        StubProtocol.lock.withLock {
            StubProtocol.answers["https://tv.home/api/v1/auth/token"] = .response(
                status: 401, body: #"{"error":{"code":"invalid_credentials"}}"#)
        }
        let request = HttpRequest(
            method: "POST", url: "https://tv.home/api/v1/auth/token",
            headers: [HttpHeader(name: "Authorization", value: "Bearer t")], body: #"{"a":1}"#)

        let output = await executor.perform(request)

        #expect(output == .http(HttpResponse(status: 401, body: #"{"error":{"code":"invalid_credentials"}}"#)))
        let sent = StubProtocol.lock.withLock { StubProtocol.seen.last }
        #expect(sent?.httpMethod == "POST")
        #expect(sent?.value(forHTTPHeaderField: "Authorization") == "Bearer t")
        #expect(sent?.httpBody == Data(#"{"a":1}"#.utf8))
    }

    @Test(arguments: [
        (URLError.Code.timedOut, HttpFailureKind.timeout),
        (.notConnectedToInternet, .offline),
        (.cannotFindHost, .offline),
        (.secureConnectionFailed, .tls),
        (.serverCertificateUntrusted, .tls),
        (.badServerResponse, .other),
    ])
    func failuresMapToTheCoresKinds(code: URLError.Code, kind: HttpFailureKind) async {
        let url = "https://fail.home/\(code.rawValue)"
        StubProtocol.lock.withLock { StubProtocol.answers[url] = .failure(code) }
        let output = await executor.perform(HttpRequest(method: "GET", url: url, headers: [], body: nil))
        guard case .httpFailed(let failure) = output else {
            Issue.record("expected a failure, got \(output)")
            return
        }
        #expect(failure.kind == kind)
    }

    @Test func aMalformedUrlFailsWithoutARequest() async {
        let output = await executor.perform(HttpRequest(method: "GET", url: "", headers: [], body: nil))
        guard case .httpFailed(let failure) = output else {
            Issue.record("expected a failure")
            return
        }
        #expect(failure.kind == .other)
    }
}

@MainActor
struct TimerExecutorTests {
    @Test func aOneShotTimerFiresOnce() async throws {
        let timers = TimerExecutor()
        let fired = Counter()
        timers.start(id: 1, request: TimerRequest(afterMs: 10, repeat: false)) { fired.add() }
        try await fired.reach(1)
        try await Task.sleep(for: .milliseconds(50))
        #expect(fired.value == 1)
        #expect(timers.activeCount == 0)
    }

    @Test func aRepeatingTimerFiresUntilCancelled() async throws {
        let timers = TimerExecutor()
        let fired = Counter()
        timers.start(id: 7, request: TimerRequest(afterMs: 10, repeat: true)) { fired.add() }
        try await fired.reach(3)
        timers.cancel(id: 7)
        let atCancel = fired.value
        #expect(atCancel >= 3)
        try await Task.sleep(for: .milliseconds(50))
        #expect(fired.value == atCancel)
        #expect(timers.activeCount == 0)
    }

    @Test func aCancelledTimerNeverFires() async throws {
        let timers = TimerExecutor()
        let fired = Counter()
        timers.start(id: 2, request: TimerRequest(afterMs: 20, repeat: false)) { fired.add() }
        timers.cancel(id: 2)
        try await Task.sleep(for: .milliseconds(60))
        #expect(fired.value == 0)
    }
}

@MainActor
struct SocketExecutorTests {
    private func firstOutputs(_ url: String) async -> [EffectOutput] {
        let sockets = SocketExecutor()
        return await withCheckedContinuation { continuation in
            var outputs: [EffectOutput] = []
            sockets.open(id: 1, request: SocketOpen(url: url, headers: [])) { output in
                outputs.append(output)
                if case .socketClosed = output {
                    continuation.resume(returning: outputs)
                }
            }
        }
    }

    @Test func anUnreachableSocketClosesAbnormallyWithoutOpening() async {
        // nothing listens on the discard port
        let outputs = await firstOutputs("ws://127.0.0.1:9/couch")
        #expect(outputs == [.socketClosed(SocketClosed(code: 1006, reason: nil))])
    }

    @Test func aMalformedUrlClosesAtOnce() async {
        let outputs = await firstOutputs("")
        guard case .socketClosed(let closed) = outputs.first else {
            Issue.record("expected a close, got \(outputs)")
            return
        }
        #expect(closed.code == 1006)
    }
}

@MainActor
final class Counter {
    private(set) var value = 0
    func add() { value += 1 }

    /// Waits for `count` (generously: a busy CI machine runs timers late, never early).
    func reach(_ count: Int) async throws {
        for _ in 0..<300 where value < count {
            try await Task.sleep(for: .milliseconds(10))
        }
    }
}

struct StoreTests {
    @Test func aFileStoreRoundTripsAndForgets() throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let store = FileStore(directory: directory)

        #expect(try store.read("servers") == nil)
        try store.write("servers", value: "[1]")
        try store.write("servers", value: "[1,2]")
        #expect(try store.read("servers") == "[1,2]")
        try store.delete("servers")
        try store.delete("servers")
        #expect(try store.read("servers") == nil)
    }

    @Test func fileStoreKeysCannotEscapeTheDirectory() {
        let store = FileStore(directory: URL(filePath: "/tmp/cv"))
        #expect(store.url(for: "../../etc/passwd").deletingLastPathComponent().path() == "/tmp/cv/")
        #expect(store.url(for: "token.a/b").lastPathComponent == "token%2Ea%2Fb.json")
    }

    @Test func aDefaultsStoreRoundTrips() throws {
        let suite = "io.stepes.couchverse.tests.\(UUID().uuidString)"
        defer { UserDefaults().removePersistentDomain(forName: suite) }
        let store = DefaultsStore(suiteName: suite)
        try store.write("accounts", value: "{}")
        #expect(try store.read("accounts") == "{}")
        try store.delete("accounts")
        #expect(try store.read("accounts") == nil)
    }

    @Test func storeEffectsReportFailuresToTheCore() {
        let output = FailingStore().execute(StoreRequest(key: "servers", op: .read))
        guard case .storeFailed = output else {
            Issue.record("expected a failure, got \(output)")
            return
        }
        #expect(MemoryStore(["k": "v"]).execute(StoreRequest(key: "k", op: .read)) == .stored(StoredValue(value: "v")))
        #expect(MemoryStore().execute(StoreRequest(key: "k", op: .write("v"))) == .storeDone)
    }

    @Test(.enabled(if: ProcessInfo.processInfo.environment["CI"] == nil, "CI keychains may be locked"))
    func theKeychainKeepsTokens() throws {
        let store = KeychainStore(service: "io.stepes.couchverse.tests.\(UUID().uuidString)")
        defer { try? store.delete("token.a") }
        #expect(try store.read("token.a") == nil)
        try store.write("token.a", value: "tok-1")
        try store.write("token.a", value: "tok-2")
        #expect(try store.read("token.a") == "tok-2")
        try store.delete("token.a")
        #expect(try store.read("token.a") == nil)
    }
}
