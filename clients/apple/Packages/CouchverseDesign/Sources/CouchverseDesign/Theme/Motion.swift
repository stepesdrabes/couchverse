import SwiftUI

extension View {
    /// Animates changes of `value` with `animation`, or with a short ease instead of a spring when
    /// the viewer turns on Reduce Motion (plan 12.4): what changes still eases into place, without
    /// overshooting or bouncing.
    public func motion(_ animation: Animation, value: some Equatable) -> some View {
        modifier(MotionModifier(animation: animation, value: value))
    }
}

private struct MotionModifier<Value: Equatable>: ViewModifier {
    let animation: Animation
    let value: Value
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func body(content: Content) -> some View {
        content.animation(reduceMotion ? .easeInOut(duration: 0.25) : animation, value: value)
    }
}
