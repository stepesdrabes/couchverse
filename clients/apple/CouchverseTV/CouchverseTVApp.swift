import CouchverseCore
import CouchverseFeatures
import SwiftUI

@main
struct CouchverseTVApp: App {
    @State private var app = LiveRuntime.make()

    var body: some Scene {
        WindowGroup {
            CouchverseRoot()
                .environment(app.runtime)
                .environment(app.player)
                .environment(app.requests)
        }
    }
}
