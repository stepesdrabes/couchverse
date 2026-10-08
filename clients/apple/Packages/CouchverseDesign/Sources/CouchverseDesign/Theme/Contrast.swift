import SwiftUI
import UIKit

// The palette's secondary text meets WCAG AA on every surface; a viewer who turns on Increase
// Contrast asks for more, so screens draw secondary text and dividing lines with these, which
// brighten towards the text colour then. The tokens themselves stay the design's values.
extension Tokens.Palette {
    /// `muted`, for secondary text.
    public static let mutedText = Color(muted, raised: 0.45)
    /// `faint`, for the quietest text.
    public static let faintText = Color(faint, raised: 0.45)
    /// `edge`, for the lines that set a field, a card or a track apart.
    public static let edgeLine = Color(edge, raised: 0.35)
}

extension Color {
    /// `standard`, mixed `fraction` of the way towards the text colour under Increase Contrast.
    fileprivate init(_ standard: Color, raised fraction: Double) {
        let base = UIColor(standard)
        let raised = UIColor(standard.mix(with: Tokens.Palette.text, by: fraction))
        self.init(uiColor: UIColor { $0.accessibilityContrast == .high ? raised : base })
    }
}
