import SwiftUI
import UIKit

extension Tokens.TypeRole {
    public var font: Font { .system(textStyle, weight: weight) }
}

extension View {
    /// Applies a type-ramp role: the system text style (so Dynamic Type scales it, tvOS included)
    /// with the role's weight and tracking.
    public func typeRole(_ role: Tokens.TypeRole) -> some View {
        modifier(TypeRoleModifier(role: role))
    }
}

private struct TypeRoleModifier: ViewModifier {
    let role: Tokens.TypeRole
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    func body(content: Content) -> some View {
        content
            .font(role.font)
            .tracking(role.trackingEm * pointSize)
    }

    /// Tracking is set in ems, so it needs the size the style resolves to right now.
    private var pointSize: CGFloat {
        guard role.trackingEm != 0 else { return 0 }
        let traits = UITraitCollection(preferredContentSizeCategory: UIContentSizeCategory(dynamicTypeSize))
        return UIFont.preferredFont(forTextStyle: role.textStyle.uiKit, compatibleWith: traits).pointSize
    }
}

extension Font.TextStyle {
    fileprivate var uiKit: UIFont.TextStyle {
        switch self {
        #if os(tvOS)
            case .largeTitle: .title1
        #else
            case .largeTitle: .largeTitle
        #endif
        case .title: .title1
        case .title2: .title2
        case .title3: .title3
        case .headline: .headline
        case .subheadline: .subheadline
        case .callout: .callout
        case .footnote: .footnote
        case .caption: .caption1
        case .caption2: .caption2
        default: .body
        }
    }
}
