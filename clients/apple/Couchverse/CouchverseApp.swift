import CouchverseCore
import CouchverseFeatures
import SwiftUI

@main
struct CouchverseApp: App {
    @State private var app = LiveRuntime.make()

    var body: some Scene {
        WindowGroup {
            CouchverseRoot()
                .environment(app.runtime)
                .environment(app.player)
        }
        // a download that ended while the app was suspended or gone wakes it: the session's
        // delegate keeps the file, then the system may suspend the app again
        .backgroundTask(.urlSession(LiveRuntime.downloadsSession)) { [downloads = app.downloads] in
            await downloads?.eventsDelivered()
        }
    }
}
