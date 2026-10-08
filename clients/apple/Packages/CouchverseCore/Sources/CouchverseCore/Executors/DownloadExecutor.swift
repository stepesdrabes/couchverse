import Foundation

/// Runs the core's `Download` effects. `deliver` resolves a start: `downloadProgress` now and
/// then, ending with `downloadFinished` or `downloadFailed`; a start the core cancelled is never
/// answered again.
@MainActor
public protocol DownloadExecuting: AnyObject {
    func start(id: UInt64, request: DownloadStart, deliver: @escaping @MainActor (EffectOutput) -> Void)
    func cancel(id: UInt64)
    func remove(name: String)
}

/// Downloads on a device that keeps none: an Apple TV, whose storage the system may reclaim at
/// any time, and tests. Every start fails.
@MainActor
public final class NoDownloads: DownloadExecuting {
    public nonisolated init() {}

    public func start(id: UInt64, request: DownloadStart, deliver: @escaping @MainActor (EffectOutput) -> Void) {
        Task { deliver(.downloadFailed(DownloadFailure(message: "this device keeps no downloads"))) }
    }

    public func cancel(id: UInt64) {}

    public func remove(name: String) {}
}

/// What a transfer did. `task` tells apart transfers of one name: a cancelled one still reports
/// its end after another has started.
public struct TransferEvent: Sendable, Equatable {
    public enum Kind: Sendable, Equatable {
        case progress(received: UInt64, total: UInt64?)
        /// The file is complete in the downloads directory.
        case finished(bytes: UInt64)
        /// The server answered with something other than the file.
        case refused(status: Int)
        /// Interrupted (no network, no space, cancelled); `resumable` when the system left data
        /// to continue from.
        case failed(message: String, noSpace: Bool, resumable: Bool)
    }

    public let name: String
    public let task: Int
    public let kind: Kind

    public init(name: String, task: Int, kind: Kind) {
        self.name = name
        self.task = task
        self.kind = kind
    }
}

/// The transfers behind `DownloadExecutor`: a background `URLSession` on the device
/// (`BackgroundTransfers`), a fake in tests.
@MainActor
public protocol DownloadTransfers: AnyObject {
    /// Every event, in the order the transfers reported them.
    var events: ((TransferEvent) -> Void)? { get set }
    /// The transfers still running by file name, including ones an earlier launch started.
    func running() async -> [String: Int]
    /// Fetches `url` into the file `name`; returns the transfer's id.
    func start(_ url: URL, name: String) -> Int
    /// Continues an interrupted transfer of `name` from the system's resume data.
    func resume(_ data: Data, name: String) -> Int
    /// Stops every transfer of `name`.
    func cancel(name: String)
}

/// The core's downloads (plan 10.8): one transfer per file the core names, into the downloads
/// directory. A start attaches to a transfer of that name that is still running (the system keeps
/// an earlier launch's going) and finishes at once when the file is already there, so the core
/// picks its downloads up again after a relaunch. An interrupted transfer continues from the
/// system's resume data, a few times on its own and then whenever the core starts it again; a
/// start without a URL does nothing else, and fails when there is nothing to pick up. Progress
/// goes out at most once a second.
@MainActor
public final class DownloadExecutor: DownloadExecuting {
    /// How often an interrupted transfer continues on its own before the core hears of it.
    static let resumes = 3
    static let progressInterval = Duration.seconds(1)

    private struct Transfer {
        let effect: UInt64
        let deliver: @MainActor (EffectOutput) -> Void
        /// The latest address the core gave; `nil` when it only asked to pick the transfer up.
        let url: URL?
        var task: Int?
        /// Continued from resume data, which asks the address it was first started with.
        var resumed = false
        var resumes = 0
        var reported: ContinuousClock.Instant?
    }

    private let transfers: any DownloadTransfers
    private let files: DownloadFiles
    private let now: @MainActor () -> ContinuousClock.Instant
    /// What the core follows, by file name.
    private var active: [String: Transfer] = [:]
    private var names: [UInt64: String] = [:]

    public init(
        transfers: any DownloadTransfers, files: DownloadFiles,
        now: @escaping @MainActor () -> ContinuousClock.Instant = { .now }
    ) {
        self.transfers = transfers
        self.files = files
        self.now = now
        transfers.events = { [weak self] event in self?.handle(event) }
    }

    public func start(id: UInt64, request: DownloadStart, deliver: @escaping @MainActor (EffectOutput) -> Void) {
        let name = request.name
        let url = URL(string: request.url)
        let previous = active.removeValue(forKey: name)
        if let previous {
            names[previous.effect] = nil
        }
        guard files.url(for: name) != nil, request.url.isEmpty || url != nil else {
            let failure = DownloadFailure(message: "cannot fetch \(request.url) into \(name)")
            Task { deliver(.downloadFailed(failure)) }
            return
        }
        if let bytes = files.size(of: name) {
            Task { deliver(.downloadFinished(DownloadFinished(bytes: bytes))) }
            return
        }
        active[name] = Transfer(
            effect: id, deliver: deliver, url: url ?? previous?.url, task: previous?.task,
            resumed: previous?.resumed ?? false)
        names[id] = name
        guard previous?.task == nil else { return }
        Task {
            let running = await transfers.running()
            guard active[name]?.effect == id else { return }
            if let task = running[name] {
                active[name]?.task = task
            } else if let bytes = files.size(of: name) {
                finish(name, .downloadFinished(DownloadFinished(bytes: bytes)))
            } else if !begin(name) {
                finish(name, .downloadFailed(DownloadFailure(message: "no transfer of \(name)")))
            }
        }
    }

    public func cancel(id: UInt64) {
        guard let name = names.removeValue(forKey: id) else { return }
        active[name] = nil
        transfers.cancel(name: name)
        files.removeResumeData(for: name)
    }

    public func remove(name: String) {
        // a transfer nobody follows (another account's, signed out of) would bring the file back
        if active[name] == nil {
            transfers.cancel(name: name)
        }
        files.remove(name)
    }

    /// Starts the transfer, from where an interrupted one stopped when it left resume data.
    private func begin(_ name: String) -> Bool {
        guard var transfer = active[name] else { return false }
        if let data = files.resumeData(for: name) {
            transfer.task = transfers.resume(data, name: name)
            transfer.resumed = true
        } else if let url = transfer.url {
            transfer.task = transfers.start(url, name: name)
            transfer.resumed = false
        } else {
            return false
        }
        active[name] = transfer
        return true
    }

    private func handle(_ event: TransferEvent) {
        let name = event.name
        // a transfer picked up before the session listed it is known by its first event
        guard var transfer = active[name], transfer.task == nil || transfer.task == event.task else { return }
        transfer.task = event.task
        active[name] = transfer
        switch event.kind {
        case .progress(let received, let total):
            let now = now()
            if let reported = transfer.reported, reported.duration(to: now) < Self.progressInterval {
                return
            }
            active[name]?.reported = now
            transfer.deliver(.downloadProgress(DownloadProgress(receivedBytes: received, totalBytes: total)))
        case .finished(let bytes):
            finish(name, .downloadFinished(DownloadFinished(bytes: bytes)))
        case .refused(let status):
            // resume data asks the address the transfer started with, whose grant may have
            // expired since; the core's latest address starts it over
            if transfer.resumed {
                files.removeResumeData(for: name)
                if transfer.url != nil && begin(name) {
                    return
                }
            }
            finish(name, .downloadFailed(DownloadFailure(message: "the server answered \(status)")))
        case .failed(let message, let noSpace, let resumable):
            if !resumable {
                // resume data this failed from would only fail the same way again
                files.removeResumeData(for: name)
            } else if !noSpace && transfer.resumes < Self.resumes {
                active[name]?.resumes += 1
                if begin(name) {
                    return
                }
            }
            finish(name, .downloadFailed(DownloadFailure(message: message, noSpace: noSpace)))
        }
    }

    /// Ends the transfer the core follows with its terminal output.
    private func finish(_ name: String, _ output: EffectOutput) {
        guard let transfer = active.removeValue(forKey: name) else { return }
        names[transfer.effect] = nil
        transfer.deliver(output)
    }
}
