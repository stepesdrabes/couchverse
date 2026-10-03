import Foundation

/// One-shot and repeating timers on the continuous clock, so a pairing deadline keeps counting
/// while the device sleeps. Repeats are scheduled from deadlines rather than from the previous
/// tick, so they never drift.
@MainActor
public final class TimerExecutor: TimerScheduling {
    private var running: [UInt64: Task<Void, Never>] = [:]

    public init() {}

    public var activeCount: Int { running.count }

    public func start(id: UInt64, request: TimerRequest, fire: @escaping @MainActor () -> Void) {
        let interval = Duration.milliseconds(Int64(clamping: request.afterMs))
        let repeats = request.repeat ?? false
        running[id] = Task { [weak self] in
            let clock = ContinuousClock()
            var deadline = clock.now + interval
            while !Task.isCancelled {
                do {
                    try await Task.sleep(until: deadline, clock: clock)
                } catch {
                    return
                }
                fire()
                guard repeats else { break }
                deadline += interval
            }
            self?.running[id] = nil
        }
    }

    public func cancel(id: UInt64) {
        running.removeValue(forKey: id)?.cancel()
    }
}
