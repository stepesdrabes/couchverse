import CouchverseCore
import CouchverseDesign
import Foundation
import Testing

@testable import CouchverseFeatures

@MainActor
struct DownloadLabelTests {
    init() { L10n.language = "en" }

    @Test func eachStateReadsAsOnTheOtherClients() {
        #expect(
            Fixtures.downloads(.loaded).items.map(DownloadLabels.state) == [
                "Downloading \u{00B7} 42%", "Preparing \u{00B7} 65%", "Waiting for the server",
                "Not enough space on this device", "Downloaded \u{00B7} 2.25 GB",
            ])
    }

    @Test func sizesAndPercentsFollowTheDisplayLanguage() {
        L10n.language = "cs"
        #expect(DownloadLabels.size(bytes: 2_254_857_830) == "2,25 GB")
        #expect(DownloadLabels.size(bytes: 912_000_000) == "912 MB")
        let fetching = Fixtures.downloads(.loaded).items[0]
        #expect(DownloadLabels.state(fetching) == "Stahuje se \u{00B7} 42 %")
        L10n.language = "en"
        #expect(DownloadLabels.size(bytes: 1_500_000_000) == "1.5 GB")
    }

    @Test(arguments: [(-0.2, "0"), (0.4299, "42"), (0.999, "99"), (1, "100"), (1.7, "100")])
    func percentsAreWholeAndNeverPastTheEnd(fraction: Double, percent: String) {
        #expect(DownloadLabels.percent(fraction) == percent)
    }

    @Test func everyFailureCodeHasItsWords() {
        #expect(DownloadLabels.failure("unsupported") == L10n.downloadFailedUnsupported)
        #expect(DownloadLabels.failure("expired") == L10n.downloadFailedExpired)
        #expect(DownloadLabels.failure("prepare_failed") == L10n.downloadFailedPrepare)
        #expect(DownloadLabels.failure("no_space") == L10n.downloadFailedNoSpace)
        #expect(DownloadLabels.failure("fetch_failed") == L10n.downloadFailedFetch)
        #expect(DownloadLabels.failure(nil) == L10n.downloadFailedFetch)
    }

    @Test func qualitiesAndNames() {
        #expect(DownloadLabels.qualities.map(DownloadLabels.quality) == ["Original", "1080p", "720p", "480p"])
        let items = Fixtures.downloads(.loaded).items
        #expect(DownloadLabels.name(items[0]) == "Couch Tales \u{00B7} The Long Night In")
        #expect(DownloadLabels.name(items[4]) == "Glass Harbor")
    }

    @Test func theDownloadNoticesHaveTheirWords() {
        #expect(CatalogLabels.notice("download_unsupported") == L10n.downloadFailedUnsupported)
        #expect(CatalogLabels.notice("downloads_disabled") == L10n.downloadsDisabled)
        #expect(CatalogLabels.notice("download_failed") == L10n.problemGeneric)
    }
}
