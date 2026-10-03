import SwiftUI

/// The canvas behind heroes and onboarding: soft orbs of the accent plus the brand violet and
/// teal over the dark background, drifting slowly. Radial gradients rather than blurs, because a
/// TV GPU cannot afford a large blur every frame. A new tint cross-fades in.
public struct GlowBackdrop: View {
    let tint: Color
    let intensity: Double

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.ambience) private var ambience

    public init(tint: Color, intensity: Double = 1) {
        self.tint = tint
        self.intensity = intensity
    }

    public var body: some View {
        if ambience == .flat {
            Tokens.Palette.bg.ignoresSafeArea()
        } else {
            glow
        }
    }

    private var glow: some View {
        let still = reduceMotion || ambience == .still
        return TimelineView(.animation(minimumInterval: 1 / 30, paused: still)) { timeline in
            let t = still ? 0 : timeline.date.timeIntervalSinceReferenceDate
            GeometryReader { geometry in
                let size = geometry.size
                let extent = max(size.width, size.height)
                ZStack {
                    Tokens.Palette.bg
                    orb(Tokens.Palette.glowViolet, opacity: 0.22, diameter: extent * 0.9)
                        .position(drift(t, period: 41, phase: 0, center: CGPoint(x: 0.1, y: 0.05), in: size))
                    orb(Tokens.Palette.glowTeal, opacity: 0.16, diameter: extent * 0.85)
                        .position(drift(t, period: 53, phase: 2, center: CGPoint(x: 0.95, y: 0.95), in: size))
                    orb(tint, opacity: 0.5, diameter: extent * 0.8)
                        .position(drift(t, period: 37, phase: 4, center: CGPoint(x: 0.75, y: 0.15), in: size))
                        .id(tint.description)
                        .transition(.opacity)
                }
                .animation(.easeInOut(duration: 0.8), value: tint.description)
            }
        }
        .ignoresSafeArea()
        .accessibilityHidden(true)
    }

    private func orb(_ color: Color, opacity: Double, diameter: CGFloat) -> some View {
        Circle()
            .fill(
                RadialGradient(
                    colors: [color.opacity(opacity * intensity), color.opacity(0)],
                    center: .center, startRadius: 0, endRadius: diameter / 2)
            )
            .frame(width: diameter, height: diameter)
    }

    /// A slow Lissajous wander of a few percent around `center` (in unit coordinates).
    private func drift(
        _ t: Double, period: Double, phase: Double, center: CGPoint, in size: CGSize
    )
        -> CGPoint
    {
        let angle = t / period * 2 * .pi + phase
        return CGPoint(
            x: (center.x + 0.06 * sin(angle)) * size.width,
            y: (center.y + 0.05 * cos(angle * 0.8)) * size.height)
    }
}
