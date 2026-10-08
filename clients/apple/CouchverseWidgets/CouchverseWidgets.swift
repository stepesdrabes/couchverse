import CouchverseShared
import SwiftUI
import WidgetKit

/// The iPhone and iPad app's extension (plan 10.9): Continue Watching on the home screen and the
/// couch Live Activity. It reads what the app leaves in the App Group and never runs the core.
@main
struct CouchverseWidgets: WidgetBundle {
    var body: some Widget {
        ContinueWatchingWidget()
        CouchLiveActivity()
    }
}

extension Color {
    /// The session's accent as the app wrote it, else the core's default one.
    init(accent hex: String?) {
        let color = hex.flatMap(HexColor.init) ?? HexColor("#e50914")!
        self.init(red: color.red, green: color.green, blue: color.blue)
    }
}
