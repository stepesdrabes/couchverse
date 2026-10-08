import Foundation

/// Downloads in a background `URLSession`: the system carries a transfer on while the app is
/// suspended or gone, wakes the app when it ends and reconnects it to the session's tasks on the
/// next launch. Tasks are named by their file (`taskDescription`, which the system keeps with
/// them), so the executor finds an earlier launch's transfers by name.
@MainActor
public final class BackgroundTransfers: DownloadTransfers {
    public let identifier: String
    public let files: DownloadFiles
    private let session: URLSession
    private let delegate: SessionDelegate

    public var events: ((TransferEvent) -> Void)? {
        get { delegate.events }
        set { delegate.events = newValue }
    }

    public init(identifier: String, files: DownloadFiles, userAgent: String) {
        self.identifier = identifier
        self.files = files
        let configuration = URLSessionConfiguration.background(withIdentifier: identifier)
        // the viewer asked for it: start now rather than when the system finds it convenient
        configuration.isDiscretionary = false
        configuration.sessionSendsLaunchEvents = true
        configuration.httpShouldSetCookies = false
        configuration.httpCookieAcceptPolicy = .never
        configuration.httpAdditionalHeaders = ["User-Agent": userAgent]
        delegate = SessionDelegate(files: files)
        // the main queue keeps the events in order and on the actor the executor runs on
        session = URLSession(configuration: configuration, delegate: delegate, delegateQueue: .main)
    }

    public func running() async -> [String: Int] {
        var running: [String: Int] = [:]
        for task in await session.allTasks where task.state == .running || task.state == .suspended {
            if let name = task.taskDescription {
                running[name] = task.taskIdentifier
            }
        }
        return running
    }

    public func start(_ url: URL, name: String) -> Int {
        named(session.downloadTask(with: url), name)
    }

    public func resume(_ data: Data, name: String) -> Int {
        named(session.downloadTask(withResumeData: data), name)
    }

    public func cancel(name: String) {
        session.getAllTasks { tasks in
            for task in tasks where task.taskDescription == name {
                task.cancel()
            }
        }
    }

    /// Returns once the system has handed over the events it kept for this session while the app
    /// was not running; the app's background task for the session ends then.
    public func eventsDelivered() async {
        await delegate.eventsDelivered()
    }

    private func named(_ task: URLSessionDownloadTask, _ name: String) -> Int {
        task.taskDescription = name
        task.resume()
        return task.taskIdentifier
    }
}

/// The session's callbacks, on the main queue. A finished file has to be moved before its
/// callback returns (the system deletes it then), so that happens here.
@MainActor
final class SessionDelegate: NSObject, URLSessionDownloadDelegate {
    var events: ((TransferEvent) -> Void)?
    private let files: DownloadFiles
    /// The system delivered its kept events before the app's background task asked.
    private var delivered = false
    private var waiting: [CheckedContinuation<Void, Never>] = []

    init(files: DownloadFiles) {
        self.files = files
    }

    func eventsDelivered() async {
        if delivered {
            delivered = false
            return
        }
        // the system's time for a background launch is short: never hold it past that
        Task {
            try? await Task.sleep(for: .seconds(25))
            release()
        }
        await withCheckedContinuation { waiting.append($0) }
    }

    private func release() {
        let waiting = waiting
        self.waiting = []
        waiting.forEach { $0.resume() }
    }

    nonisolated func urlSession(
        _ session: URLSession, downloadTask: URLSessionDownloadTask, didWriteData bytesWritten: Int64,
        totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64
    ) {
        let total = totalBytesExpectedToWrite > 0 ? UInt64(totalBytesExpectedToWrite) : nil
        report(downloadTask, .progress(received: UInt64(max(totalBytesWritten, 0)), total: total))
    }

    nonisolated func urlSession(
        _ session: URLSession, downloadTask: URLSessionDownloadTask, didResumeAtOffset fileOffset: Int64,
        expectedTotalBytes: Int64
    ) {
        let total = expectedTotalBytes > 0 ? UInt64(expectedTotalBytes) : nil
        report(downloadTask, .progress(received: UInt64(max(fileOffset, 0)), total: total))
    }

    nonisolated func urlSession(
        _ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL
    ) {
        guard let name = downloadTask.taskDescription else { return }
        let status = (downloadTask.response as? HTTPURLResponse)?.statusCode
        report(downloadTask, files.finished(location, as: name, status: status))
    }

    nonisolated func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: (any Error)?) {
        // a transfer that finished was reported with its file
        guard let error, let name = task.taskDescription else { return }
        report(task, files.interrupted(name, by: error))
    }

    nonisolated func urlSessionDidFinishEvents(forBackgroundURLSession session: URLSession) {
        MainActor.assumeIsolated {
            if waiting.isEmpty {
                delivered = true
            } else {
                release()
            }
        }
    }

    private nonisolated func report(_ task: URLSessionTask, _ kind: TransferEvent.Kind) {
        guard let name = task.taskDescription else { return }
        let event = TransferEvent(name: name, task: task.taskIdentifier, kind: kind)
        MainActor.assumeIsolated { events?(event) }
    }
}
