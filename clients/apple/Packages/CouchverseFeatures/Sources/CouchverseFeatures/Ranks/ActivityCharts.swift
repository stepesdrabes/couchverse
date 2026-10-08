import CouchverseCore
import CouchverseDesign
import SwiftUI

/// A heatmap's last weeks as columns of seven days, Monday at the top and the newest week on the
/// right, with a month label over each column where a month begins.
struct HeatmapLayout: Equatable {
    struct Day: Equatable {
        let date: Date
        let seconds: UInt64
        let level: UInt8
    }

    struct Month: Equatable {
        let column: Int
        let date: Date
    }

    /// Seven slots each; empty before the map's first day and after its last (today).
    let columns: [[Day?]]
    let months: [Month]

    /// The server's dates are calendar days; reading them in GMT keeps every day 24 hours long.
    static let calendar: Calendar = {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = .gmt
        return calendar
    }()

    init(_ heatmap: Heatmap, weeks: Int) {
        guard let start = try? Date(heatmap.from, strategy: .iso8601.year().month().day()) else {
            columns = []
            months = []
            return
        }
        let calendar = Self.calendar
        let lead = (calendar.component(.weekday, from: start) + 5) % 7
        var slots = [Day?](repeating: nil, count: lead)
        for (offset, day) in heatmap.days.enumerated() {
            let date = calendar.date(byAdding: .day, value: offset, to: start) ?? start
            slots.append(Day(date: date, seconds: day.seconds, level: day.level))
        }
        slots += [Day?](repeating: nil, count: (7 - slots.count % 7) % 7)
        let weeksOfDays = stride(from: 0, to: slots.count, by: 7).map { Array(slots[$0..<$0 + 7]) }
        columns = Array(weeksOfDays.suffix(weeks))
        months = Self.months(columns)
    }

    /// A label where a column's first day falls in a new month; the first column's only when the
    /// next label leaves it room.
    static func months(_ columns: [[Day?]]) -> [Month] {
        var marks: [Month] = []
        var previous: Int?
        for (index, column) in columns.enumerated() {
            guard let first = column.lazy.compactMap({ $0 }).first else { continue }
            let month = calendar.component(.month, from: first.date)
            if month != previous {
                marks.append(Month(column: index, date: first.date))
                previous = month
            }
        }
        if marks.count > 1, marks[1].column - marks[0].column < 3 {
            marks.removeFirst()
        }
        return marks
    }
}

/// The day's 24 hours as wedges of a clock face: midnight at the top and the day running
/// clockwise, each wedge as long as its hour's share of the busiest one.
enum WatchClockLayout {
    struct Wedge: Equatable {
        let hour: Int
        /// Degrees clockwise from three o'clock, as SwiftUI measures arcs.
        let start: Double
        let end: Double
        /// 0 for an hour with nothing watched, 1 for the busiest.
        let fraction: Double
    }

    static func wedges(_ hours: [UInt64], gap: Double = 2) -> [Wedge] {
        let peak = hours.max() ?? 0
        return hours.prefix(24).enumerated().map { hour, seconds in
            Wedge(
                hour: hour, start: Double(hour) * 15 - 90 + gap / 2, end: Double(hour + 1) * 15 - 90 - gap / 2,
                fraction: peak == 0 ? 0 : Double(seconds) / Double(peak))
        }
    }

    /// The hour watched most, the first of a tie; nil when nothing was.
    static func busiest(_ hours: [UInt64]) -> Int? {
        guard let peak = hours.max(), peak > 0 else { return nil }
        return hours.firstIndex(of: peak)
    }

    /// Where a label for `hour` sits: the middle of its wedge, `radius` from the centre.
    static func labelPoint(hour: Int, center: CGPoint, radius: CGFloat) -> CGPoint {
        let angle = (Double(hour) * 15 + 7.5 - 90) * .pi / 180
        return CGPoint(x: center.x + radius * cos(angle), y: center.y + radius * sin(angle))
    }
}

/// The last 26 weeks of activity, every day as bright as the core rated it against this member's
/// own active days, with the months above and the scale below.
struct HeatmapView: View {
    let heatmap: Heatmap

    @Environment(\.accent) private var accent
    @Environment(\.colorSchemeContrast) private var contrast
    @State private var width: CGFloat = 0
    @ScaledMetric(relativeTo: .caption) private var monthsHeight: CGFloat = 18

    static let weeks = 26
    /// How bright each level is over the empty cell's surface; the quiet levels stand further
    /// apart with Increase Contrast.
    static let levelOpacity: [Double] = [0, 0.22, 0.45, 0.7, 1]
    static let raisedLevelOpacity: [Double] = [0, 0.4, 0.6, 0.8, 1]

    private var levels: [Double] { contrast == .increased ? Self.raisedLevelOpacity : Self.levelOpacity }

    private var layout: HeatmapLayout { HeatmapLayout(heatmap, weeks: Self.weeks) }
    private var maxPitch: CGFloat { Idiom.isTV ? 40 : 26 }

    var body: some View {
        let layout = layout
        let pitch = width / CGFloat(Self.weeks)
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            Canvas { context, _ in
                draw(layout, pitch: pitch, in: &context)
            }
            .frame(height: monthsHeight + 7 * pitch)
            .frame(maxWidth: CGFloat(Self.weeks) * maxPitch)
            .onGeometryChange(for: CGFloat.self) {
                $0.size.width
            } action: {
                width = $0
            }
            .accessibilityElement()
            .accessibilityLabel(L10n.profilesActivityHeading)
            .accessibilityValue(summary)
            ViewThatFits(in: .horizontal) {
                HStack(spacing: Tokens.Spacing.lg) {
                    summaryText
                    Spacer(minLength: 0)
                    legend
                }
                VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                    summaryText
                    legend
                }
            }
        }
    }

    private var summary: String {
        L10n.profilesActivitySummary(
            hours: RanksWords.number(heatmap.totalSeconds / 3600), days: Int(heatmap.activeDays))
    }

    private var summaryText: some View {
        Text(summary)
            .typeRole(Tokens.TypeRamp.caption)
            .foregroundStyle(Tokens.Palette.mutedText)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityHidden(true)
    }

    private var legend: some View {
        HStack(spacing: Tokens.Spacing.xs) {
            Text(L10n.profilesActivityLess)
            ForEach(0..<levels.count, id: \.self) { level in
                RoundedRectangle(cornerRadius: 2)
                    .fill(Tokens.Palette.surface2)
                    .overlay { RoundedRectangle(cornerRadius: 2).fill(accent.color.opacity(levels[level])) }
                    .frame(width: 10, height: 10)
            }
            Text(L10n.profilesActivityMore)
        }
        .typeRole(Tokens.TypeRamp.caption)
        .foregroundStyle(Tokens.Palette.faintText)
        .accessibilityHidden(true)
    }

    /// A shorter run than the grid holds keeps the cells' size and ends on the right, today.
    private func draw(_ layout: HeatmapLayout, pitch: CGFloat, in context: inout GraphicsContext) {
        guard pitch > 0 else { return }
        let cell = pitch * 0.8
        let corner = pitch * 0.2
        let start = CGFloat(Self.weeks - layout.columns.count) * pitch
        var month = Date.FormatStyle().month(.abbreviated).locale(L10n.locale)
        month.timeZone = .gmt
        for mark in layout.months {
            let label = context.resolve(
                Text(mark.date.formatted(month))
                    .font(Tokens.TypeRamp.caption.font)
                    .foregroundStyle(Tokens.Palette.faintText))
            // a month starting in this week stays inside the grid
            let width = label.measure(in: CGSize(width: CGFloat.infinity, height: monthsHeight)).width
            let x = min(start + CGFloat(mark.column) * pitch, CGFloat(Self.weeks) * pitch - width)
            context.draw(label, at: CGPoint(x: x, y: 0), anchor: .topLeading)
        }
        for (column, days) in layout.columns.enumerated() {
            for (row, day) in days.enumerated() {
                guard let day else { continue }
                let rect = CGRect(
                    x: start + CGFloat(column) * pitch, y: monthsHeight + CGFloat(row) * pitch, width: cell,
                    height: cell)
                let shape = Path(roundedRect: rect, cornerRadius: corner)
                context.fill(shape, with: .color(Tokens.Palette.surface2))
                let opacity = levels[min(Int(day.level), levels.count - 1)]
                if opacity > 0 {
                    context.fill(shape, with: .color(accent.color.opacity(opacity)))
                }
            }
        }
    }
}

/// When a member watches: a 24-slice radial histogram, midnight at the top.
struct WatchClockView: View {
    let hours: [UInt64]

    @Environment(\.accent) private var accent

    var body: some View {
        let wedges = WatchClockLayout.wedges(hours)
        Canvas { context, size in
            let center = CGPoint(x: size.width / 2, y: size.height / 2)
            let labelInset: CGFloat = Idiom.isTV ? 40 : 22
            let outer = min(size.width, size.height) / 2 - labelInset
            let inner = outer * 0.35
            context.stroke(
                Path(ellipseIn: CGRect(x: center.x - outer, y: center.y - outer, width: outer * 2, height: outer * 2)),
                with: .color(Tokens.Palette.edgeLine), lineWidth: 1)
            for wedge in wedges where wedge.fraction > 0 {
                let radius = inner + (outer - inner) * wedge.fraction
                var path = Path()
                path.addArc(
                    center: center, radius: radius, startAngle: .degrees(wedge.start), endAngle: .degrees(wedge.end),
                    clockwise: false)
                path.addArc(
                    center: center, radius: inner, startAngle: .degrees(wedge.end), endAngle: .degrees(wedge.start),
                    clockwise: true)
                path.closeSubpath()
                context.fill(path, with: .color(accent.color))
            }
            for hour in [0, 6, 12, 18] {
                let label = Text(String(hour))
                    .font(Tokens.TypeRamp.caption.font)
                    .foregroundStyle(Tokens.Palette.faintText)
                let point = WatchClockLayout.labelPoint(hour: hour, center: center, radius: outer + labelInset / 2)
                context.draw(label, at: point)
            }
        }
        .aspectRatio(1, contentMode: .fit)
        .frame(maxWidth: Idiom.isTV ? 420 : 260)
        .accessibilityElement()
        .accessibilityLabel(L10n.profilesClockHeading)
        .accessibilityValue(value)
    }

    /// The busiest hour and the time watched in all.
    private var value: String {
        guard let busiest = WatchClockLayout.busiest(hours) else { return L10n.profilesClockEmpty }
        let total = RanksWords.watchTime(seconds: hours.reduce(0, +))
        return "\(L10n.profilesClockHour(hour: String(busiest))), \(total)"
    }
}
