import CouchverseCore
import CouchverseDesign
import SwiftUI

extension View {
    /// While the server is out of reach the downloads are all there is to watch, so on iPhone and
    /// iPad they take the place of the tabs until it answers again; a TV keeps no downloads.
    func offlineDownloads() -> some View {
        modifier(OfflineDownloads())
    }
}

private struct OfflineDownloads: ViewModifier {
    @Environment(CoreRuntime.self) private var core

    func body(content: Content) -> some View {
        #if os(iOS)
            if core.session.offline {
                NavigationStack {
                    DownloadsScreen(offline: true)
                        .toolbar {
                            ToolbarItem(placement: .topBarTrailing) { AccountButton() }
                        }
                }
            } else {
                content
            }
        #else
            content
        #endif
    }
}

/// The way to the downloads in Settings on iPhone and iPad, while the server offers them or the
/// device keeps some.
struct DownloadsLink: View {
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        #if os(iOS)
            if core.session.features.downloads || !core.downloads.items.isEmpty {
                NavigationLink {
                    DownloadsScreen()
                } label: {
                    Label(L10n.downloadsTitle, systemImage: "arrow.down.circle")
                }
            }
        #endif
    }
}
