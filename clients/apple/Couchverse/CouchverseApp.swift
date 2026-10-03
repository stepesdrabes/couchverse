import CouchverseCore
import CouchverseFeatures
import SwiftUI

@main
struct CouchverseApp: App {
    @State private var runtime = LiveRuntime.make()

    var body: some Scene {
        WindowGroup {
            CouchverseRoot()
                .environment(runtime)
        }
    }
}
