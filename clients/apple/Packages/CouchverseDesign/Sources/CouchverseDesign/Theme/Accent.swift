import CouchverseCore
import SwiftUI

/// The colours a surface derives from one accent. The core does the maths (6.4), so a title or
/// profile tints the same on every client; this only turns its hex strings into colours.
public struct Accent: Equatable, Sendable {
    public let color: Color
    public let strong: Color
    public let soft: Color
    public let onAccent: Color
    /// The accent lightened to read as text (WCAG AA) on every surface: the colour of
    /// accent-coloured text, never `color`.
    public let ink: Color

    public init(_ palette: AccentPalette) {
        color = Color(hex: palette.accent) ?? Tokens.Palette.accent
        strong = Color(hex: palette.strong) ?? Tokens.Palette.accentStrong
        soft = Color(hex: palette.soft) ?? Tokens.Palette.accentSoft
        onAccent = Color(hex: palette.onAccent) ?? Tokens.Palette.onAccent
        ink = Color(hex: palette.ink) ?? color
    }

    public static let standard = Accent(.fallback)
}

extension EnvironmentValues {
    @Entry public var accent: Accent = .standard
}

extension View {
    /// Tints this subtree with `palette`: controls via `tint`, custom views via `\.accent`. The
    /// tint is the ink, since tinted controls mostly draw text (links, menus, plain buttons);
    /// filled buttons set the accent itself.
    public func accent(_ palette: AccentPalette) -> some View {
        let accent = Accent(palette)
        return environment(\.accent, accent).tint(accent.ink)
    }
}

extension Color {
    /// `#rrggbb` or `#rrggbbaa`, as the core and the API send colours.
    public init?(hex: String) {
        let digits = hex.trimmingCharacters(in: .whitespaces).trimmingCharacters(in: ["#"])
        guard digits.count == 6 || digits.count == 8, let value = UInt64(digits, radix: 16) else {
            return nil
        }
        let rgba = digits.count == 6 ? value << 8 | 0xFF : value
        self.init(
            .sRGB,
            red: Double((rgba >> 24) & 0xFF) / 255,
            green: Double((rgba >> 16) & 0xFF) / 255,
            blue: Double((rgba >> 8) & 0xFF) / 255,
            opacity: Double(rgba & 0xFF) / 255
        )
    }
}
