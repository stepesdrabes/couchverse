import SwiftUI

public enum Idiom {
    public static var isTV: Bool {
        #if os(tvOS)
            true
        #else
            false
        #endif
    }
}

extension View {
    /// Centres a column of content at a comfortable reading width on iPad and TV.
    public func readableWidth(_ width: CGFloat = 560) -> some View {
        frame(maxWidth: width).frame(maxWidth: .infinity)
    }

    /// Lifts a custom focusable view the way tvOS lifts system controls; nothing on touch devices.
    public func focusLift(_ focused: Bool, scale: CGFloat = 1.08) -> some View {
        modifier(FocusLift(focused: focused, scale: scale))
    }
}

private struct FocusLift: ViewModifier {
    let focused: Bool
    let scale: CGFloat
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func body(content: Content) -> some View {
        if Idiom.isTV {
            content
                .scaleEffect(focused && !reduceMotion ? scale : 1)
                .shadow(color: .black.opacity(focused ? 0.45 : 0), radius: focused ? 24 : 0, y: focused ? 16 : 0)
                .animation(reduceMotion ? nil : Tokens.Motion.snappy, value: focused)
        } else {
            content
        }
    }
}
