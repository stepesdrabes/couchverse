import SwiftUI

/// How much of the ambient decoration (backdrop glow and drift, skeleton shimmer) is drawn.
public enum Ambience: Sendable {
    case live
    /// Drawn but still, for screenshots.
    case still
    /// No glow, no motion and solid buttons instead of glass ones: what a snapshot without a host
    /// app can render (a prominent glass button at the largest text sizes draws nothing), in
    /// references that stay small and change only with the layout.
    case flat
}

extension EnvironmentValues {
    @Entry public var ambience = Ambience.live
}

/// A placeholder block in the shape of the content it stands for, with a slow shimmer.
public struct Skeleton: View {
    let width: CGFloat?
    let height: CGFloat?
    /// Width over height, for a block as wide as it is offered: a grid cell's card.
    let aspectRatio: CGFloat?
    let shape: AnyShape

    public init(width: CGFloat? = nil, height: CGFloat, cornerRadius: CGFloat = Tokens.Radius.input) {
        self.width = width
        self.height = height
        aspectRatio = nil
        shape = AnyShape(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
    }

    public init(aspectRatio: CGFloat, cornerRadius: CGFloat = Tokens.Radius.input) {
        width = nil
        height = nil
        self.aspectRatio = aspectRatio
        shape = AnyShape(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
    }

    public init(circle diameter: CGFloat) {
        width = diameter
        height = diameter
        aspectRatio = nil
        shape = AnyShape(Circle())
    }

    public var body: some View {
        let block =
            shape
            .fill(Tokens.Palette.surface2)
            .overlay { Shimmer().clipShape(shape) }
        Group {
            if let aspectRatio {
                block.aspectRatio(aspectRatio, contentMode: .fit)
            } else {
                block
                    .frame(width: width, height: height)
                    .frame(maxWidth: width == nil ? .infinity : nil, alignment: .leading)
            }
        }
        .accessibilityHidden(true)
    }
}

private struct Shimmer: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.ambience) private var ambience

    var body: some View {
        if reduceMotion || ambience != .live {
            Color.clear
        } else {
            TimelineView(.animation(minimumInterval: 1 / 30)) { timeline in
                let phase = timeline.date.timeIntervalSinceReferenceDate.truncatingRemainder(dividingBy: 1.6) / 1.6
                GeometryReader { geometry in
                    LinearGradient(
                        colors: [.clear, Tokens.Palette.edgeLine.opacity(0.6), .clear],
                        startPoint: .leading, endPoint: .trailing
                    )
                    .frame(width: geometry.size.width * 0.6)
                    .offset(x: (phase * 1.6 - 0.6) * geometry.size.width)
                }
            }
        }
    }
}
