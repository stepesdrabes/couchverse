import CouchverseCore
import SwiftUI

/// Where a card or a link in the catalog leads, pushed onto the tab's navigation stack.
enum CatalogRoute: Hashable {
    case title(slug: String)
    case browse(BrowseKey)
}

extension View {
    /// Tells the core this screen shows `surface` while it is on screen: the core loads it, or
    /// shows what it has cached as stale and refreshes it. The open and the close always pair up,
    /// because they bracket the screen's task. The core catches a visited home, title or My List
    /// up with the viewer's other devices itself; a `showsProgress` screen also reloads when the
    /// player over it closes, since it stayed open meanwhile and the progress just changed.
    func coreScreen(_ surface: Surface, showsProgress: Bool = false) -> some View {
        modifier(CoreScreen(surface: surface, showsProgress: showsProgress))
    }

    /// Pull to refresh on touch devices; the spinner stays until the core's answer lands.
    func catalogRefreshable(_ surface: Surface, busy: @escaping () -> Bool) -> some View {
        modifier(CatalogRefreshable(surface: surface, busy: busy))
    }

    func catalogDestinations() -> some View {
        navigationDestination(for: CatalogRoute.self) { route in
            switch route {
            case .title(let slug): TitleScreen(slug: slug)
            case .browse(let key): BrowseScreen(key: key)
            }
        }
    }
}

private struct CoreScreen: ViewModifier {
    let surface: Surface
    let showsProgress: Bool
    @Environment(CoreRuntime.self) private var core

    func body(content: Content) -> some View {
        content
            .task(id: surface) {
                core.send(.screenOpened(surface))
                // suspended for as long as the screen is up; cancelled when it goes away
                try? await Task.sleep(for: .seconds(60 * 60 * 24 * 365))
                core.send(.screenClosed(surface))
            }
            .onChange(of: core.player.target == nil) { _, closed in
                if showsProgress && closed {
                    core.send(.refreshRequested(surface))
                }
            }
    }
}

private struct CatalogRefreshable: ViewModifier {
    let surface: Surface
    let busy: () -> Bool
    @Environment(CoreRuntime.self) private var core

    func body(content: Content) -> some View {
        #if os(iOS)
            content.refreshable {
                core.send(.refreshRequested(surface))
                // the render that marks it loading is decoded off the main actor
                try? await Task.sleep(for: .milliseconds(150))
                for _ in 0..<80 where busy() {
                    try? await Task.sleep(for: .milliseconds(100))
                }
            }
        #else
            content
        #endif
    }
}
