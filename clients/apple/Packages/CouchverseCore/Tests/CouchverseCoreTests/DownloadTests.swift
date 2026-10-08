import Foundation
import Testing

@testable import CouchverseCore

/// Transfers that only move when a test says so, listed as running until they end.
@MainActor
final class FakeTransfers: DownloadTransfers {
    struct Started: Equatable {
        let name: String
        let task: Int
        /// The address, or `nil` when it continued from resume data.
        let url: URL?
    }

    var events: ((TransferEvent) -> Void)?
    /// What the session lists as running, by name.
    var tasks: [String: Int] = [:]
    private(set) var started: [Started] = []
    private(set) var cancelled: [String] = []
    private var lastTask = 0

    func running() async -> [String: Int] { tasks }

    func start(_ url: URL, name: String) -> Int {
        begin(name, url: url)
    }

    func resume(_ data: Data, name: String) -> Int {
        begin(name, url: nil)
    }

    func cancel(name: String) {
        cancelled.append(name)
        tasks[name] = nil
    }

    /// Reports `kind` for the latest transfer of `name`.
    func emit(_ name: String, _ kind: TransferEvent.Kind) {
        let task = tasks[name] ?? started.last { $0.name == name }?.task ?? 0
        if case .progress = kind {
        } else {
            tasks[name] = nil
        }
        events?(TransferEvent(name: name, task: task, kind: kind))
    }

    private func begin(_ name: String, url: URL?) -> Int {
        lastTask += 1
        started.append(Started(name: name, task: lastTask, url: url))
        tasks[name] = lastTask
        return lastTask
    }
}

/// A clock the test moves.
@MainActor
final class ManualClock {
    private let origin = ContinuousClock.now
    var elapsed = Duration.zero

    var now: ContinuousClock.Instant { origin + elapsed }
}

@MainActor
struct DownloadExecutorTests {
    let directory = FileManager.default.temporaryDirectory.appending(path: "downloads-\(UUID().uuidString)")
    let transfers = FakeTransfers()
    let clock = ManualClock()
    let outputs = Outputs()

    var files: DownloadFiles { DownloadFiles(directory: directory) }

    func executor() -> DownloadExecutor {
        DownloadExecutor(transfers: transfers, files: files) { [clock] in clock.now }
    }

    func start(_ executor: DownloadExecutor, id: UInt64 = 1, url: String = Self.url, name: String = "d1.mp4") {
        executor.start(id: id, request: DownloadStart(url: url, name: name)) { [outputs] in outputs.add(id, $0) }
    }

    static let url = "https://tv.home/api/v1/media/g1/downloads/f1"

    func write(_ name: String, bytes: Int) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Data(repeating: 7, count: bytes).write(to: directory.appending(path: name))
    }

    /// Lets the executor's own tasks run.
    func settle() async {
        for _ in 0..<20 {
            await Task.yield()
        }
    }

    @Test func aStartFetchesTheFileAndReportsProgressAtMostOnceASecond() async {
        let executor = executor()
        start(executor)
        await settle()
        #expect(transfers.started == [FakeTransfers.Started(name: "d1.mp4", task: 1, url: URL(string: Self.url))])

        transfers.emit("d1.mp4", .progress(received: 100, total: 1000))
        clock.elapsed = .milliseconds(500)
        transfers.emit("d1.mp4", .progress(received: 200, total: 1000))
        clock.elapsed = .milliseconds(1100)
        transfers.emit("d1.mp4", .progress(received: 300, total: nil))
        transfers.emit("d1.mp4", .finished(bytes: 1000))
        transfers.emit("d1.mp4", .progress(received: 1000, total: 1000))

        #expect(
            outputs.of(1) == [
                .downloadProgress(DownloadProgress(receivedBytes: 100, totalBytes: 1000)),
                .downloadProgress(DownloadProgress(receivedBytes: 300, totalBytes: nil)),
                .downloadFinished(DownloadFinished(bytes: 1000)),
            ])
    }

    @Test func aFileAlreadyThereFinishesAtOnce() async throws {
        try write("d1.mp4", bytes: 5)
        let executor = executor()
        start(executor, url: "")
        await settle()
        #expect(outputs.of(1) == [.downloadFinished(DownloadFinished(bytes: 5))])
        #expect(transfers.started.isEmpty)
    }

    @Test func withoutAnAddressAStartPicksUpARunningTransferOrFails() async {
        transfers.tasks["d1.mp4"] = 41
        let executor = executor()
        start(executor, url: "")
        start(executor, id: 2, url: "", name: "d2.mp4")
        await settle()
        #expect(transfers.started.isEmpty)
        #expect(outputs.of(2) == [.downloadFailed(DownloadFailure(message: "no transfer of d2.mp4"))])

        transfers.emit("d1.mp4", .finished(bytes: 900))
        #expect(outputs.of(1) == [.downloadFinished(DownloadFinished(bytes: 900))])
    }

    @Test func aSecondStartOfANameFollowsTheTransferAlreadyRunning() async {
        let executor = executor()
        start(executor)
        await settle()
        start(executor, id: 2, url: "")
        await settle()
        transfers.emit("d1.mp4", .finished(bytes: 10))
        #expect(transfers.started.count == 1)
        #expect(outputs.of(1).isEmpty)
        #expect(outputs.of(2) == [.downloadFinished(DownloadFinished(bytes: 10))])
    }

    @Test func anInterruptedTransferContinuesFromItsResumeDataAFewTimes() async {
        let executor = executor()
        start(executor)
        await settle()
        for attempt in 1...DownloadExecutor.resumes {
            files.saveResumeData(Data("resume-\(attempt)".utf8), for: "d1.mp4")
            transfers.emit("d1.mp4", .failed(message: "connection lost", noSpace: false, resumable: true))
        }
        #expect(transfers.started.map(\.url) == [URL(string: Self.url), nil, nil, nil])
        #expect(outputs.of(1).isEmpty)

        transfers.emit("d1.mp4", .failed(message: "connection lost", noSpace: false, resumable: true))
        #expect(outputs.of(1) == [.downloadFailed(DownloadFailure(message: "connection lost", noSpace: false))])
        #expect(transfers.started.count == 4)
    }

    @Test func aFullDiskIsReportedAtOnce() async {
        let executor = executor()
        start(executor)
        await settle()
        files.saveResumeData(Data("resume".utf8), for: "d1.mp4")
        transfers.emit("d1.mp4", .failed(message: "disk full", noSpace: true, resumable: true))
        #expect(outputs.of(1) == [.downloadFailed(DownloadFailure(message: "disk full", noSpace: true))])
        #expect(transfers.started.count == 1)
    }

    @Test func resumeDataTheServerRefusesStartsOverFromTheLatestAddress() async {
        files.saveResumeData(Data("resume".utf8), for: "d1.mp4")
        let executor = executor()
        start(executor)
        await settle()
        #expect(transfers.started.map(\.url) == [nil], "it continues where the last launch stopped")

        // the grant in the address the transfer first used has expired
        transfers.emit("d1.mp4", .refused(status: 403))
        #expect(transfers.started.map(\.url) == [nil, URL(string: Self.url)])
        #expect(files.resumeData(for: "d1.mp4") == nil)
        #expect(outputs.of(1).isEmpty)

        transfers.emit("d1.mp4", .refused(status: 404))
        #expect(outputs.of(1) == [.downloadFailed(DownloadFailure(message: "the server answered 404"))])
    }

    @Test func resumeDataThatLeadsNowhereIsDropped() async {
        files.saveResumeData(Data("from an older system".utf8), for: "d1.mp4")
        let executor = executor()
        start(executor)
        await settle()
        transfers.emit("d1.mp4", .failed(message: "cannot resume", noSpace: false, resumable: false))
        #expect(outputs.of(1) == [.downloadFailed(DownloadFailure(message: "cannot resume", noSpace: false))])
        #expect(files.resumeData(for: "d1.mp4") == nil)

        // the core asks again: from the start
        start(executor, id: 2)
        await settle()
        #expect(transfers.started.map(\.url) == [nil, URL(string: Self.url)])
    }

    @Test func aCancelledTransferStopsAndIsNeverAnsweredAgain() async {
        let executor = executor()
        start(executor)
        await settle()
        let task = transfers.started[0].task
        files.saveResumeData(Data("resume".utf8), for: "d1.mp4")

        executor.cancel(id: 1)
        #expect(transfers.cancelled == ["d1.mp4"])
        #expect(files.resumeData(for: "d1.mp4") == nil)

        transfers.events?(
            TransferEvent(
                name: "d1.mp4", task: task, kind: .failed(message: "cancelled", noSpace: false, resumable: false)))
        #expect(outputs.of(1).isEmpty)
    }

    @Test func aCancelWhileTheSessionIsAskedStartsNothing() async {
        let executor = executor()
        start(executor)
        executor.cancel(id: 1)
        await settle()
        #expect(transfers.started.isEmpty)
        #expect(outputs.of(1).isEmpty)
    }

    @Test func removingDeletesTheFileAndStopsATransferNobodyFollows() throws {
        try write("d1.jpg", bytes: 3)
        files.saveResumeData(Data("resume".utf8), for: "d1.jpg")
        let executor = executor()
        executor.remove(name: "d1.jpg")
        #expect(files.size(of: "d1.jpg") == nil)
        #expect(files.resumeData(for: "d1.jpg") == nil)
        #expect(transfers.cancelled == ["d1.jpg"])
    }

    @Test func aNameOutsideTheDirectoryIsRefused() async {
        let executor = executor()
        start(executor, name: "../d1.mp4")
        await settle()
        guard case .downloadFailed = outputs.of(1).first else {
            Issue.record("expected a failure, got \(outputs.of(1))")
            return
        }
        #expect(transfers.started.isEmpty)
    }

    @Test func aDeviceWithoutDownloadsFailsEveryStart() async {
        let none = NoDownloads()
        none.start(id: 3, request: DownloadStart(url: Self.url, name: "d1.mp4")) { [outputs] in outputs.add(3, $0) }
        await settle()
        guard case .downloadFailed = outputs.of(3).first else {
            Issue.record("expected a failure, got \(outputs.of(3))")
            return
        }
    }
}

/// What each start was answered, by effect id.
@MainActor
final class Outputs {
    private var outputs: [(UInt64, EffectOutput)] = []

    func add(_ id: UInt64, _ output: EffectOutput) {
        outputs.append((id, output))
    }

    func of(_ id: UInt64) -> [EffectOutput] {
        outputs.filter { $0.0 == id }.map(\.1)
    }
}

struct DownloadFilesTests {
    let files = DownloadFiles(
        directory: FileManager.default.temporaryDirectory.appending(path: "downloads-\(UUID().uuidString)"))

    private func fetched(bytes: Int) throws -> URL {
        let location = FileManager.default.temporaryDirectory.appending(path: "fetched-\(UUID().uuidString)")
        try Data(repeating: 1, count: bytes).write(to: location)
        return location
    }

    @Test func aFetchedFileIsKeptOutOfBackups() throws {
        defer { try? FileManager.default.removeItem(at: files.directory) }
        let location = try fetched(bytes: 12)
        files.saveResumeData(Data("resume".utf8), for: "d1.mp4")

        #expect(files.finished(location, as: "d1.mp4", status: 206) == .finished(bytes: 12))
        #expect(files.size(of: "d1.mp4") == 12)
        #expect(!FileManager.default.fileExists(atPath: location.path()))
        #expect(files.resumeData(for: "d1.mp4") == nil)
        let kept = try #require(files.url(for: "d1.mp4"))
        #expect(try kept.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)

        // a later copy of the same download replaces it
        #expect(files.finished(try fetched(bytes: 20), as: "d1.mp4", status: 200) == .finished(bytes: 20))
    }

    @Test func anythingButTheFileIsTheServerRefusing() throws {
        let location = try fetched(bytes: 40)
        defer { try? FileManager.default.removeItem(at: location) }
        #expect(files.finished(location, as: "d1.mp4", status: 403) == .refused(status: 403))
        #expect(files.finished(location, as: "d1.mp4", status: nil) == .refused(status: 0))
        #expect(files.size(of: "d1.mp4") == nil)
    }

    @Test func namesNeverLeaveTheDirectory() {
        #expect(files.url(for: "d1.mp4") == files.directory.appending(path: "d1.mp4"))
        for name in ["", "../d1.mp4", "a/b.mp4", ".", "..", ".hidden"] {
            #expect(files.url(for: name) == nil, "\(name)")
        }
        #expect(files.size(of: "../etc/hosts") == nil)
    }

    @Test func anInterruptionKeepsWhatTheSystemLeftToContinueFrom() {
        defer { try? FileManager.default.removeItem(at: files.directory) }
        let data = Data("resume-data".utf8)
        let lost = URLError(.networkConnectionLost, userInfo: [NSURLSessionDownloadTaskResumeData: data])
        guard case .failed(_, let noSpace, let resumable) = files.interrupted("d1.mp4", by: lost) else {
            Issue.record("expected a failure")
            return
        }
        #expect(!noSpace)
        #expect(resumable)
        #expect(files.resumeData(for: "d1.mp4") == data)

        guard case .failed(_, _, let again) = files.interrupted("d2.mp4", by: URLError(.timedOut)) else {
            Issue.record("expected a failure")
            return
        }
        #expect(!again)
    }

    @Test(arguments: [
        (NSError(domain: NSCocoaErrorDomain, code: CocoaError.fileWriteOutOfSpace.rawValue), true),
        (
            NSError(
                domain: NSURLErrorDomain, code: URLError.unknown.rawValue,
                userInfo: [NSUnderlyingErrorKey: NSError(domain: NSPOSIXErrorDomain, code: Int(ENOSPC))]), true
        ),
        (NSError(domain: NSURLErrorDomain, code: URLError.cannotWriteToFile.rawValue), true),
        (NSError(domain: NSURLErrorDomain, code: URLError.timedOut.rawValue), false),
        (NSError(domain: NSPOSIXErrorDomain, code: Int(EACCES)), false),
    ])
    func aFullDiskIsToldApart(error: NSError, noSpace: Bool) {
        #expect(DownloadFiles.isNoSpace(error) == noSpace)
    }
}

/// The background session's callbacks, called the way the system calls them.
@MainActor
struct SessionDelegateTests {
    let files = DownloadFiles(
        directory: FileManager.default.temporaryDirectory.appending(path: "downloads-\(UUID().uuidString)"))
    let session = URLSession(configuration: .ephemeral)

    func task(_ name: String?) -> URLSessionDownloadTask {
        let task = session.downloadTask(with: URL(filePath: "/downloads/f1"))
        task.taskDescription = name
        return task
    }

    @Test func callbacksBecomeEventsOfTheNamedFile() throws {
        defer { try? FileManager.default.removeItem(at: files.directory) }
        let delegate = SessionDelegate(files: files)
        var events: [TransferEvent] = []
        delegate.events = { events.append($0) }
        let download = task("d1.mp4")
        let id = download.taskIdentifier

        delegate.urlSession(session, downloadTask: download, didResumeAtOffset: 200, expectedTotalBytes: 1000)
        delegate.urlSession(
            session, downloadTask: download, didWriteData: 100, totalBytesWritten: 300,
            totalBytesExpectedToWrite: NSURLSessionTransferSizeUnknown)
        // a task without a name is none of the core's
        delegate.urlSession(
            session, downloadTask: task(nil), didWriteData: 1, totalBytesWritten: 1, totalBytesExpectedToWrite: 1)
        #expect(
            events == [
                TransferEvent(name: "d1.mp4", task: id, kind: .progress(received: 200, total: 1000)),
                TransferEvent(name: "d1.mp4", task: id, kind: .progress(received: 300, total: nil)),
            ])

        let resume = Data("resume".utf8)
        let lost = URLError(.networkConnectionLost, userInfo: [NSURLSessionDownloadTaskResumeData: resume])
        delegate.urlSession(session, task: download, didCompleteWithError: lost)
        // a task that finished was reported with its file
        delegate.urlSession(session, task: download, didCompleteWithError: nil)
        #expect(events.count == 3)
        guard case .failed(_, false, true) = events.last?.kind else {
            Issue.record("expected a resumable failure, got \(String(describing: events.last))")
            return
        }
        #expect(files.resumeData(for: "d1.mp4") == resume)

        // without an answer from the server there is no file to keep
        let location = FileManager.default.temporaryDirectory.appending(path: "fetched-\(UUID().uuidString)")
        try Data("x".utf8).write(to: location)
        defer { try? FileManager.default.removeItem(at: location) }
        delegate.urlSession(session, downloadTask: download, didFinishDownloadingTo: location)
        #expect(events.last == TransferEvent(name: "d1.mp4", task: id, kind: .refused(status: 0)))
        #expect(files.size(of: "d1.mp4") == nil)
    }

    @Test func theAppsBackgroundTaskEndsOnceTheSystemHasDeliveredItsEvents() async {
        let delegate = SessionDelegate(files: files)
        // delivered before the app asked
        delegate.urlSessionDidFinishEvents(forBackgroundURLSession: session)
        await delegate.eventsDelivered()

        // asked before they were delivered
        let waiting = Task { await delegate.eventsDelivered() }
        for _ in 0..<20 {
            await Task.yield()
        }
        delegate.urlSessionDidFinishEvents(forBackgroundURLSession: session)
        await waiting.value
    }
}

/// Starts the runtime hands its download executor, for the test to answer.
@MainActor
final class ManualDownloads: DownloadExecuting {
    struct Started {
        let id: UInt64
        let request: DownloadStart
        let deliver: @MainActor (EffectOutput) -> Void
    }

    private(set) var started: [Started] = []
    private(set) var cancelled: [UInt64] = []
    private(set) var removed: [String] = []

    func start(id: UInt64, request: DownloadStart, deliver: @escaping @MainActor (EffectOutput) -> Void) {
        started.append(Started(id: id, request: request, deliver: deliver))
    }

    func cancel(id: UInt64) {
        cancelled.append(id)
    }

    func remove(name: String) {
        removed.append(name)
    }
}

/// A download through the real core: asked for, prepared by the server, fetched by the executor.
@MainActor
struct DownloadRuntimeTests {
    let http = FakeHTTP()
    let secure = MemoryStore()
    let downloads = ManualDownloads()

    static let ready = """
        {"id":"d1","kind":"movie","titleId":"t1","titleSlug":"glass-harbor","title":"Glass Harbor",\
        "posterId":"p1","posterVer":4,"backdropId":null,"thumbId":null,"quality":"720p","status":"ready",\
        "progress":100,"durationSeconds":2400.0,"createdAt":"2026-10-02T12:00:00Z",\
        "expiresAt":"2026-10-05T12:00:00Z","audio":[{"lang":"en","label":"English"}],"subtitles":[],\
        "url":"/api/v1/media/gr/downloads/f1","sizeBytes":1000}
        """

    func signedIn() async throws -> CoreRuntime {
        var executors = Executors(
            http: http, timers: ManualTimers(), sockets: SocketExecutor(), player: SilentPlayer(), secureStore: secure,
            store: MemoryStore(Payload.signedIn))
        executors.downloads = downloads
        let config = CoreConfig(
            platform: .ios, authMode: .bearer, deviceName: "Nora's iPhone", locale: "cs-CZ", origin: "")
        let runtime = try CoreRuntime(config: config, executors: executors, now: { 1_000 })
        try secure.write("token.\(Payload.accountId)", value: "tok-1")
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(1, "admin"))
        runtime.start()
        await runtime.untilIdle()
        runtime.send(.capabilitiesReported(.avPlayer(DeviceProfileTests.appleTV4K)))
        await runtime.untilIdle()
        return runtime
    }

    @Test func aDownloadIsFetchedByTheExecutorAndPublished() async throws {
        let runtime = try await signedIn()
        #expect(runtime.downloads.status == .loaded)
        http.route("POST", "http://tv.home/api/v1/me/downloads?lang=cs", Self.ready)

        runtime.send(.downloadRequested(DownloadAsk(target: PlayTarget(kind: .movie, id: "t1"), quality: .hd720)))
        await runtime.untilIdle()

        let transfer = try #require(downloads.started.first)
        #expect(transfer.request == DownloadStart(url: "http://tv.home/api/v1/media/gr/downloads/f1", name: "d1.mp4"))
        #expect(runtime.downloads.items.map(\.state) == [.fetching])

        transfer.deliver(.downloadProgress(DownloadProgress(receivedBytes: 250, totalBytes: 1000)))
        await runtime.untilIdle()
        #expect(runtime.downloads.items.first?.progress == 0.25)

        transfer.deliver(.downloadFinished(DownloadFinished(bytes: 1000)))
        await runtime.untilIdle()
        #expect(runtime.downloads.items.map(\.state) == [.ready])
        #expect(runtime.downloads.usedBytes == 1000)
        // its artwork is kept beside it, for the list without a network
        #expect(downloads.started.map(\.request.name) == ["d1.mp4", "d1.jpg"])

        runtime.send(.downloadRemoved(DownloadRef(id: "d1")))
        await runtime.untilIdle()
        #expect(downloads.removed == ["d1.mp4", "d1.jpg"])
        #expect(runtime.downloads.items.isEmpty)
    }
}
