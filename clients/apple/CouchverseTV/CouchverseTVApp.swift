import CouchverseCore
import CouchverseFeatures
import SwiftUI

@main
struct CouchverseTVApp: App {
    @State private var runtime = LiveRuntime.make()

    var body: some Scene {
        WindowGroup {
            CouchverseRoot()
                .environment(runtime)
        }
    }
}
