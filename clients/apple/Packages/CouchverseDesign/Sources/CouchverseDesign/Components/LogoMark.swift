import CouchverseShared
import SwiftUI

/// The app mark, as the web and Android draw it: the logo in the on-accent colour across 80 percent
/// of a rounded square of the accent, which follows the server's accent once signed in. Decorative,
/// like the web's.
public struct LogoMark: View {
    let size: CGFloat
    @Environment(\.accent) private var accent

    public init(size: CGFloat) {
        self.size = size
    }

    public var body: some View {
        CouchverseLogo()
            .fill(accent.onAccent)
            .frame(width: size * 0.8, height: size * 0.8)
            .frame(width: size, height: size)
            .background(accent.color, in: .rect(cornerRadius: size / 4, style: .continuous))
            .accessibilityHidden(true)
    }
}
