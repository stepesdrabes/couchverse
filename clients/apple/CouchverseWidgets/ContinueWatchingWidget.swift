import CouchverseShared
import SwiftUI
import WidgetKit

/// Continue Watching on the home screen: the unfinished titles with how far in, each a tap from
/// playing where it stopped. The app reloads it whenever it writes a new shelf snapshot.
struct ContinueWatchingWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: SharedContainer.continueWidgetKind, provider: ContinueProvider()) { entry in
            ContinueWatchingView(snapshot: entry.snapshot)
        }
        .configurationDisplayName(Text("widget_continue_name"))
        .description(Text("widget_continue_description"))
        .supportedFamilies([.systemSmall, .systemMedium])
    }
}

nonisolated struct ContinueEntry: TimelineEntry {
    let date: Date
    /// `nil` until the app has written one.
    let snapshot: ShelfSnapshot?
}

nonisolated struct ContinueProvider: TimelineProvider {
    func placeholder(in context: Context) -> ContinueEntry {
        ContinueEntry(date: .now, snapshot: nil)
    }

    func getSnapshot(in context: Context, completion: @escaping @Sendable (ContinueEntry) -> Void) {
        completion(ContinueEntry(date: .now, snapshot: ShelfSnapshot.current()))
    }

    func getTimeline(in context: Context, completion: @escaping @Sendable (Timeline<ContinueEntry>) -> Void) {
        let entry = ContinueEntry(date: .now, snapshot: ShelfSnapshot.current())
        completion(Timeline(entries: [entry], policy: .never))
    }
}

/// The heading and the first one or three titles in progress; the words are the app's, in its
/// display language, and the extension's own (the system's language) before the app wrote any.
struct ContinueWatchingView: View {
    let snapshot: ShelfSnapshot?
    @Environment(\.widgetFamily) private var family
    @ScaledMetric(relativeTo: .headline) private var logoWidth: CGFloat = 24

    private var items: [ShelfSnapshot.ContinueItem] {
        Array((snapshot?.continueWatching ?? []).prefix(family == .systemSmall ? 1 : 3))
    }

    private var accent: Color { Color(accent: snapshot?.accent) }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Label {
                words(snapshot?.words.continueWatching, else: "widget_continue_name")
                    .font(.headline)
                    .lineLimit(1)
            } icon: {
                LogoGlyph(color: accent, width: logoWidth)
                    .widgetAccentable()
            }
            if items.isEmpty {
                words(snapshot?.words.nothingToContinue, else: "widget_continue_empty")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            ForEach(items) { item in
                if let link = URL(string: item.playLink) {
                    Link(destination: link) { ContinueRow(item: item, accent: accent) }
                }
            }
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .environment(\.colorScheme, .dark)
        .containerBackground(for: .widget) {
            LinearGradient(
                colors: [accent.opacity(0.35), Color(red: 0.03, green: 0.03, blue: 0.05)],
                startPoint: .topLeading, endPoint: .center)
        }
        // a small widget takes one tap anywhere
        .widgetURL(items.first.flatMap { URL(string: $0.playLink) })
    }

    private func words(_ text: String?, else key: LocalizedStringKey) -> Text {
        text.map { Text(verbatim: $0) } ?? Text(key)
    }
}

private struct ContinueRow: View {
    let item: ShelfSnapshot.ContinueItem
    let accent: Color

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(verbatim: [item.name, item.label].compactMap { $0 }.joined(separator: " \u{00B7} "))
                .font(.subheadline)
                .lineLimit(1)
            ProgressView(value: min(max(item.progress, 0), 1))
                .tint(accent)
                .accessibilityHidden(true)
        }
    }
}
