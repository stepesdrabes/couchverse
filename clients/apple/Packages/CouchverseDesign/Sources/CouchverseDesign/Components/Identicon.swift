import SwiftUI

/// The web's placeholder avatar (minidenticons 4): a mirrored 5x5 pixel pattern whose hue comes
/// from the same hash, so a profile without a picture looks identical on every client.
public struct Identicon: Equatable, Sendable {
    /// Row-major cells of the 5x5 grid.
    public let cells: [Bool]
    /// Degrees, one of nine evenly spaced hues.
    public let hue: Double

    public init(seed: String) {
        let hash = Self.hash(seed)
        hue = Double(hash % 9) * 40
        var cells = Array(repeating: false, count: 25)
        if !seed.isEmpty {
            // the JS draws cell i at column i / 5 (mirrored from 15 on) and row i % 5
            for i in 0..<25 where hash & (1 << (i % 15)) != 0 {
                let column = i > 14 ? 7 - i / 5 : i / 5
                cells[(i % 5) * 5 + column] = true
            }
        }
        self.cells = cells
    }

    /// The pixels' colour: `hsl(hue 95% 45%)`.
    public var color: Color {
        Color(hue: hue / 360, hslSaturation: 0.95, lightness: 0.45)
    }

    /// minidenticons' `simpleHash`, reproducing JavaScript's 32-bit integer coercions exactly.
    static func hash(_ seed: String) -> UInt32 {
        var hash: Int64 = 5
        for unit in seed.utf16 {
            let mixed = Int32(truncatingIfNeeded: hash) ^ Int32(unit)
            hash = Int64(mixed) * -5
        }
        return UInt32(truncatingIfNeeded: hash) >> 2
    }
}

extension Color {
    /// HSL as CSS defines it, for colours that must match the web.
    public init(hue: Double, hslSaturation saturation: Double, lightness: Double) {
        let brightness = lightness + saturation * min(lightness, 1 - lightness)
        let hsbSaturation = brightness == 0 ? 0 : 2 * (1 - lightness / brightness)
        self.init(hue: hue, saturation: hsbSaturation, brightness: brightness)
    }
}

/// Draws an identicon the way the web's SVG does: the grid inset by 1.5 cells on every side.
public struct IdenticonView: View {
    let identicon: Identicon

    public init(seed: String) {
        identicon = Identicon(seed: seed)
    }

    public var body: some View {
        Canvas { context, size in
            let cell = min(size.width, size.height) / 8
            let origin = CGPoint(
                x: (size.width - cell * 8) / 2 + cell * 1.5, y: (size.height - cell * 8) / 2 + cell * 1.5)
            for (index, filled) in identicon.cells.enumerated() where filled {
                let rect = CGRect(
                    x: origin.x + CGFloat(index % 5) * cell, y: origin.y + CGFloat(index / 5) * cell,
                    width: cell, height: cell)
                // a hair of overlap hides the seams antialiasing would draw between cells
                context.fill(Path(rect.insetBy(dx: -0.25, dy: -0.25)), with: .color(identicon.color))
            }
        }
        .background(Tokens.Palette.surface2)
        .accessibilityHidden(true)
    }
}
