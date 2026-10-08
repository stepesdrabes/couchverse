import AppIntents
import CouchverseCore
import CouchverseFeatures
import SwiftUI

@main
struct CouchverseApp: App {
    @State private var app: LiveApp

    init() {
        let app = LiveRuntime.make()
        // the intents leave their requests where the root takes links
        let requests = app.requests
        AppDependencyManager.shared.add(dependency: requests)
        _app = State(initialValue: app)
    }

    var body: some Scene {
        WindowGroup {
            CouchverseRoot()
                .environment(app.runtime)
                .environment(app.player)
                .environment(app.requests)
                .onShelfChange { CouchverseShortcuts.updateAppShortcutParameters() }
        }
        // a download that ended while the app was suspended or gone wakes it: the session's
        // delegate keeps the file, then the system may suspend the app again
        .backgroundTask(.urlSession(LiveRuntime.downloadsSession)) { [downloads = app.downloads] in
            await downloads?.eventsDelivered()
        }
    }
}
