import SwiftUI

/// The rank ring (plan 12.2): how far into the current tier, as an arc in the tier's colour around
/// an avatar, with the level in a chip at its foot. It flashes once each time `flashes` goes up (a
/// level-up), and never with Reduce Motion. Decorative: the caller says the rank in words.
public struct RankRing<Content: View>: View {
    let color: Color
    let progress: Double
    let level: String?
    let lineWidth: CGFloat
    let flashes: UInt64
    let content: Content

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    public init(
        color: Color, progress: Double, level: String? = nil, lineWidth: CGFloat, flashes: UInt64 = 0,
        @ViewBuilder content: () -> Content
    ) {
        self.color = color
        self.progress = min(max(progress, 0), 1)
        self.level = level
        self.lineWidth = lineWidth
        self.flashes = flashes
        self.content = content()
    }

    public var body: some View {
        content
            .padding(lineWidth * 1.5)
            .overlay {
                ZStack {
                    Circle().stroke(Tokens.Palette.edge, lineWidth: lineWidth)
                    Circle()
                        .trim(from: 0, to: progress)
                        .stroke(
                            LinearGradient(
                                colors: [color.opacity(0.55), color], startPoint: .topLeading, endPoint: .bottomTrailing
                            ),
                            style: StrokeStyle(lineWidth: lineWidth, lineCap: .round)
                        )
                        .rotationEffect(.degrees(-90))
                }
                .padding(lineWidth / 2)
                .accessibilityHidden(true)
            }
            .overlay { flash }
            .overlay(alignment: .bottom) {
                if let level {
                    Text(level)
                        .typeRole(Tokens.TypeRamp.eyebrow)
                        .monospacedDigit()
                        .foregroundStyle(Tokens.Accent.onAccentDark)
                        .padding(.horizontal, Tokens.Spacing.sm)
                        .padding(.vertical, Tokens.Spacing.xxs)
                        .background(color, in: Capsule())
                        .overlay { Capsule().strokeBorder(Tokens.Palette.bg, lineWidth: 2) }
                        .alignmentGuide(.bottom) { $0[VerticalAlignment.center] }
                        .accessibilityHidden(true)
                }
            }
    }

    private var flash: some View {
        let still = reduceMotion
        return Circle()
            .stroke(color, lineWidth: lineWidth / 2)
            .shadow(color: color.opacity(0.6), radius: lineWidth * 2)
            .keyframeAnimator(initialValue: RingFlash(), trigger: flashes) { ring, value in
                ring
                    .scaleEffect(still ? 1 : value.scale)
                    .opacity(still ? 0 : value.opacity)
            } keyframes: { _ in
                KeyframeTrack(\.scale) {
                    LinearKeyframe(0.92, duration: 0.01)
                    CubicKeyframe(1.5, duration: 0.7)
                }
                KeyframeTrack(\.opacity) {
                    LinearKeyframe(0.9, duration: 0.01)
                    CubicKeyframe(0, duration: 0.7)
                }
            }
            .allowsHitTesting(false)
            .accessibilityHidden(true)
    }
}

private struct RingFlash {
    var scale = 1.0
    var opacity = 0.0
}
