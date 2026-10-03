import SwiftUI

/// A round profile picture: the account's avatar through its artwork grant URL, or the identicon
/// of its username while loading, on failure and when there is none.
public struct AvatarView: View {
    let url: URL?
    let seed: String
    let name: String

    public init(url: String?, seed: String, name: String) {
        self.url = url.flatMap(URL.init(string:))
        self.seed = seed
        self.name = name
    }

    public var body: some View {
        AsyncImage(url: url, transaction: Transaction(animation: Tokens.Motion.standard)) { phase in
            if let image = phase.image {
                image.resizable().scaledToFill()
            } else {
                IdenticonView(seed: seed)
            }
        }
        .clipShape(Circle())
        .accessibilityElement()
        .accessibilityLabel(L10n.a11yProfilePicture(name: name))
        .accessibilityAddTraits(.isImage)
    }
}
