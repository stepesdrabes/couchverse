import CoreImage
import CoreImage.CIFilterBuiltins
import SwiftUI

/// A QR code a phone camera can read from across a room: dark modules on white with the quiet
/// zone the spec asks for, scaled without smoothing.
public struct QRCodeView: View {
    let payload: String
    let image: CGImage?

    public init(_ payload: String) {
        self.payload = payload
        image = Self.render(payload)
    }

    public var body: some View {
        Group {
            if let image {
                Image(decorative: image, scale: 1)
                    .resizable()
                    .interpolation(.none)
                    .scaledToFit()
                    .padding(Tokens.Spacing.md)
                    .background(.white, in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            } else {
                RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                    .fill(Tokens.Palette.surface2)
            }
        }
        .aspectRatio(1, contentMode: .fit)
        .accessibilityElement()
        .accessibilityLabel(L10n.a11yQrCode(url: payload))
        .accessibilityAddTraits(.isImage)
    }

    static func render(_ payload: String) -> CGImage? {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(payload.utf8)
        filter.correctionLevel = "M"
        guard let output = filter.outputImage else { return nil }
        return CIContext().createCGImage(output, from: output.extent)
    }
}
