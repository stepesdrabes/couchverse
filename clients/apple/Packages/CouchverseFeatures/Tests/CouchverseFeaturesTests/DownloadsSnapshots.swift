import CouchverseCore
import SwiftUI
import Testing

@testable import CouchverseFeatures

// Downloads exist on iPhone and iPad only (an Apple TV keeps none), so these render there.
#if os(iOS)
    extension ScreenSnapshots {
        @Test(arguments: ["loading", "loaded", "empty"])
        func downloads(state: String) {
            let view =
                switch state {
                case "loading": Fixtures.downloads(.loading)
                case "empty": Fixtures.downloads(.loaded, empty: true)
                default: Fixtures.downloads(.loaded)
                }
            snapshot("downloads-\(state)", Self.ready + [.downloads(view)]) {
                NavigationStack { DownloadsScreen() }
            }
        }

        /// With the server out of reach the downloads take the place of the tabs.
        @Test(arguments: ["loaded", "empty"])
        func offline(state: String) {
            let downloads = Fixtures.downloads(.loaded, empty: state == "empty")
            snapshot(
                "downloads-offline-\(state)",
                Self.ready + [.session(Fixtures.session(.stale)), .downloads(downloads)]
            ) {
                MainTabs()
            }
        }

        @Test(arguments: [DownloadState.queued, .fetching, .ready, .failed])
        func movieDownload(state: DownloadState) {
            let view = Fixtures.title(.loaded)
            snapshot(
                "title-movie-download-\(state)",
                Self.ready + [.title(view.slug, view), .downloads(Fixtures.titleDownloads(state))]
            ) {
                NavigationStack { TitleScreen(slug: view.slug) }
            }
        }

        @Test func seriesDownloads() {
            let view = Fixtures.title(.loaded, series: true)
            snapshot(
                "title-series-downloads",
                Self.ready + [.title(view.slug, view), .downloads(Fixtures.titleDownloads(nil, series: true))]
            ) {
                NavigationStack { TitleScreen(slug: view.slug) }
            }
        }
    }
#endif
