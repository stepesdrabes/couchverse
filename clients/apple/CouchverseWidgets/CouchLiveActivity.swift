import ActivityKit
import CouchverseShared
import SwiftUI
import WidgetKit

/// A couch session on the Lock Screen and in the Dynamic Island: what is on, who is there and the
/// code friends join with. The app updates it while it runs; once it stops (suspended without
/// playing) the stale date passes and the activity says so.
struct CouchLiveActivity: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: CouchActivityAttributes.self) { context in
            CouchLockScreen(content: context.state, stale: context.isStale)
                .activityBackgroundTint(Color.black.opacity(0.8))
                .activitySystemActionForegroundColor(.white)
        } dynamicIsland: { context in
            let content = context.state
            let accent = Color(accent: content.accent)
            return DynamicIsland {
                DynamicIslandExpandedRegion(.leading) {
                    Label {
                        Text(content.count, format: .number)
                    } icon: {
                        Image(systemName: "sofa.fill").foregroundStyle(accent)
                    }
                    .font(.headline)
                    .accessibilityLabel(content.membersLine)
                }
                DynamicIslandExpandedRegion(.trailing) {
                    CodeText(content: content)
                        .font(.title3.weight(.semibold))
                }
                DynamicIslandExpandedRegion(.center) {
                    Text(verbatim: content.title ?? content.heading)
                        .font(.headline)
                        .lineLimit(1)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    Text(verbatim: CouchLockScreen.line(content, stale: context.isStale))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            } compactLeading: {
                LogoGlyph(color: accent, width: 24)
            } compactTrailing: {
                Text(content.count, format: .number)
                    .monospacedDigit()
                    .accessibilityLabel(content.membersLine)
            } minimal: {
                LogoGlyph(color: accent, width: 24)
            }
            .keylineTint(accent)
        }
    }
}

private struct CouchLockScreen: View {
    let content: CouchActivityContent
    let stale: Bool
    @ScaledMetric(relativeTo: .title2) private var logoWidth: CGFloat = 34

    var body: some View {
        HStack(alignment: .center, spacing: 14) {
            LogoGlyph(color: Color(accent: content.accent), width: logoWidth)
            VStack(alignment: .leading, spacing: 2) {
                Text(verbatim: content.heading)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text(verbatim: content.title ?? content.membersLine)
                    .font(.headline)
                    .lineLimit(1)
                if let detail = content.detail {
                    Text(verbatim: detail)
                        .font(.subheadline)
                        .lineLimit(1)
                }
                Text(verbatim: Self.line(content, stale: stale))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            Spacer(minLength: 0)
            CodeText(content: content)
                .font(.title2.weight(.bold))
        }
        .padding()
        .environment(\.colorScheme, .dark)
    }

    /// Who is there, or what the session is doing when that needs saying, or that it may be out of
    /// date.
    static func line(_ content: CouchActivityContent, stale: Bool) -> String {
        if stale {
            return content.staleNote
        }
        return content.status ?? content.members.joined(separator: ", ")
    }
}

/// The six digits, read out as the code they are.
private struct CodeText: View {
    let content: CouchActivityContent

    var body: some View {
        Text(verbatim: content.code)
            .monospacedDigit()
            .accessibilityLabel(content.codeLine)
    }
}
