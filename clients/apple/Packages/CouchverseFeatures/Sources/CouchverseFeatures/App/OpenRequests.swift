import CouchverseCore
import Observation

/// Something the app was asked to open from outside it: a link, a Spotlight result, Top Shelf or
/// an App Intent.
public enum OpenRequest: Hashable, Sendable {
    case title(slug: String)
    /// Play a movie or an episode from where it stopped.
    case play(PlayTarget)
    /// The first title in Continue Watching, once the home has it.
    case continueWatching
    case myList
    /// Hand a couch code to the join screen.
    case couch(code: String)
}

extension OpenRequest {
    /// What a `couchverse://title` or `couchverse://play` link asks for (Spotlight, the intents);
    /// `nil` for any other link.
    public init?(link: String) {
        switch DeepLink(link) {
        case .title(let slug): self = .title(slug: slug)
        case .play(let target): self = .play(target)
        default: return nil
        }
    }
}

/// The request waiting for the app to act on it. It waits for an account: the signed-in screens
/// take it once they are up (after "Who's watching?" on a TV), the root takes a couch code, whose
/// join screen waits by itself. A newer request replaces one not yet taken.
@MainActor
@Observable
public final class OpenRequests {
    public private(set) var pending: OpenRequest?

    public init() {}

    public func open(_ request: OpenRequest) {
        pending = request
    }

    /// The request has been acted on; a newer one that arrived meanwhile stays.
    func finish(_ request: OpenRequest) {
        if pending == request {
            pending = nil
        }
    }
}

/// What the signed-in screens do for a request.
enum OpenStep: Equatable {
    case showTitle(slug: String)
    case showMyList
    case play(PlayTarget)
    /// Continue Watching had nothing to continue: home is the closest thing.
    case showHome
    /// The home it needs is still loading.
    case wait
}

extension OpenRequest {
    /// The step for this request with `home` as the core shows it; `nil` for a request the root
    /// takes instead.
    func step(home: HomeView) -> OpenStep? {
        switch self {
        case .title(let slug): return .showTitle(slug: slug)
        case .play(let target): return .play(target)
        case .myList: return .showMyList
        case .couch: return nil
        case .continueWatching:
            // a home still refreshing may be about to drop or reorder what it shows
            let settled =
                home.status == .loaded || home.status == .failed || home.status == .notFound
                || (home.status == .stale && home.problem != nil)
            guard settled else { return .wait }
            let first = home.rows.lazy.flatMap(\.continueWatching).first
            return first.map { .play($0.play) } ?? .showHome
        }
    }
}
