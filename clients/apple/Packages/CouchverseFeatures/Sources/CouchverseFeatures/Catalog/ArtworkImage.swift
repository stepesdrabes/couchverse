import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The core's artwork reference (a URL and its accent); SwiftUI's `Image` is the other one.
typealias Artwork = CouchverseCore.Image

/// An image from the core's artwork URLs (they carry the account's artwork grant, so the system
/// loader needs no session). While it loads, and when there is none, the space is tinted with the
/// image's extracted accent so a shelf never flashes blank.
struct ArtworkImage: View {
    let image: Artwork?
    var contentMode: ContentMode = .fill

    var body: some View {
        let tint = image?.accent.flatMap(Color.init(hex:)) ?? Tokens.Palette.surface2
        // the placeholder sets the size; a filling image overflows it and is clipped, never
        // widening the layout
        Rectangle()
            .fill(tint.opacity(0.35).gradient)
            .overlay {
                if let url = image.flatMap({ URL(string: $0.url) }) {
                    AsyncImage(url: url, transaction: Transaction(animation: Tokens.Motion.standard)) { phase in
                        if let loaded = phase.image {
                            loaded.resizable().aspectRatio(contentMode: contentMode)
                                .transition(.opacity)
                        }
                    }
                }
            }
            .clipped()
            .accessibilityHidden(true)
    }
}

/// A title's wordmark, sized by its aspect before the PNG arrives; the name stands in when there
/// is no logo or it fails to load.
struct TitleLogo: View {
    let logo: Logo?
    let name: String
    let maxWidth: CGFloat
    let maxHeight: CGFloat
    var nameRole: Tokens.TypeRole = Tokens.TypeRamp.hero

    var body: some View {
        if let logo, let url = URL(string: logo.url) {
            let aspect = logo.aspect ?? 3
            let width = min(maxWidth, maxHeight * aspect)
            AsyncImage(url: url) { phase in
                switch phase {
                case .success(let image):
                    // the stated aspect is rounded; keep the wordmark on the leading edge regardless
                    image.resizable().scaledToFit()
                        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
                case .failure: nameText
                default: Color.clear
                }
            }
            .frame(width: width, height: width / aspect, alignment: .leading)
            .accessibilityElement()
            .accessibilityLabel(name)
            .accessibilityAddTraits(.isHeader)
        } else {
            nameText
        }
    }

    private var nameText: some View {
        Text(name)
            .typeRole(nameRole)
            .foregroundStyle(Tokens.Palette.text)
            .shadow(color: .black.opacity(0.6), radius: 12)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityAddTraits(.isHeader)
    }
}
