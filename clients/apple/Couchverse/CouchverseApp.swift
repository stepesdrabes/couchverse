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
    }
}
