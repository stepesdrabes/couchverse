import Foundation

/// The `now_ms` every message carries: milliseconds since launch on the continuous clock, which
/// keeps counting while the device sleeps, so deadlines the core hands out stay truthful.
public struct MonotonicClock: Sendable {
    private let origin = ContinuousClock.now

    public init() {}

    public func nowMs() -> UInt64 {
        let elapsed = origin.duration(to: .now).components
        let ms = elapsed.seconds * 1000 + elapsed.attoseconds / 1_000_000_000_000_000
        return UInt64(max(ms, 0))
    }
}
