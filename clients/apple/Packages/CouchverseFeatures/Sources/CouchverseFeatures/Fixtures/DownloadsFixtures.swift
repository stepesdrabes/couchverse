import CouchverseCore
import Foundation

/// Downloads in every state, for the Downloads screen and the title page's download controls.
/// Their artwork is kept under names no simulator has, so the rows show the server's image, or
/// its tinted placeholder offline, the same on every run.
extension Fixtures {
    public static func download(
        _ state: DownloadState, id: String, target: PlayTarget, title: Int, episode: EpisodeNumber? = nil,
        episodeName: String? = nil, quality: DownloadQuality = .hd1080, progress: Double = 0, size: UInt64 = 0,
        problem: String? = nil
    ) -> DownloadItem {
        let source = titles[title]
        let art = episode.map { artwork("s", Int($0.episode), accent: source.accent) }
        return DownloadItem(
            id: id, target: target, title: source.name, titleSlug: slug(source.name), episode: episode,
            episodeName: episodeName, quality: quality, state: state, progress: progress, sizeBytes: size,
            image: art ?? artwork("p", title, accent: source.accent), artwork: state == .ready ? "\(id).jpg" : nil,
            problem: problem.map { Problem(code: $0, detail: "") })
    }

    /// One download in each state, newest first.
    public static func downloads(_ status: LoadStatus, empty: Bool = false) -> DownloadsView {
        guard status == .loaded, !empty else { return DownloadsView(status: status, items: [], usedBytes: 0) }
        let items = [
            download(
                .fetching, id: "d5", target: PlayTarget(kind: .episode, id: "e2"), title: 1,
                episode: EpisodeNumber(season: 1, episode: 2), episodeName: "The Long Night In", quality: .hd720,
                progress: 0.42, size: 912_000_000),
            download(
                .preparing, id: "d4", target: PlayTarget(kind: .episode, id: "e3"), title: 1,
                episode: EpisodeNumber(season: 1, episode: 3), episodeName: "Remote Control", quality: .hd720,
                progress: 0.65),
            download(.queued, id: "d3", target: PlayTarget(kind: .movie, id: "t2"), title: 2),
            download(
                .failed, id: "d2", target: PlayTarget(kind: .movie, id: "t3"), title: 3, quality: .original,
                problem: "no_space"),
            download(
                .ready, id: "d1", target: PlayTarget(kind: .movie, id: "t0"), title: 0, quality: .original,
                progress: 1, size: 2_254_857_830),
        ]
        return DownloadsView(status: .loaded, items: items, usedBytes: 2_254_857_830)
    }

    /// The title page's downloads: the movie in `state`, or for the series a finished first
    /// episode, the second coming down and the third failed.
    public static func titleDownloads(_ state: DownloadState?, series: Bool = false) -> DownloadsView {
        guard series else {
            let items = state.map { state in
                [
                    download(
                        state, id: "d1", target: PlayTarget(kind: .movie, id: "t0"), title: 0,
                        progress: state == .ready ? 1 : 0.42, size: state == .ready ? 2_254_857_830 : 0,
                        problem: state == .failed ? "fetch_failed" : nil)
                ]
            }
            return DownloadsView(status: .loaded, items: items ?? [], usedBytes: state == .ready ? 2_254_857_830 : 0)
        }
        let episode = { (number: UInt32) in EpisodeNumber(season: 1, episode: number) }
        let items = [
            download(
                .failed, id: "d3", target: PlayTarget(kind: .episode, id: "e3"), title: 1, episode: episode(3),
                problem: "expired"),
            download(
                .fetching, id: "d2", target: PlayTarget(kind: .episode, id: "e2"), title: 1, episode: episode(2),
                progress: 0.42),
            download(
                .ready, id: "d1", target: PlayTarget(kind: .episode, id: "e1"), title: 1, episode: episode(1),
                progress: 1, size: 612_000_000),
        ]
        return DownloadsView(status: .loaded, items: items, usedBytes: 612_000_000)
    }
}
