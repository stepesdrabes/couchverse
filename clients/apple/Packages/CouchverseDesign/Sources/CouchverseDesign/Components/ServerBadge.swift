import SwiftUI

/// Marks a server reached over plain http (D14: allowed, but never silently).
public struct InsecureBadge: View {
    public init() {}

    public var body: some View {
        Label(L10n.serversNotEncrypted, systemImage: "lock.open.fill")
            .typeRole(Tokens.TypeRamp.caption)
            .foregroundStyle(Tokens.Palette.danger)
            .padding(.horizontal, Tokens.Spacing.sm)
            .padding(.vertical, Tokens.Spacing.xxs)
            .background(Tokens.Palette.danger.opacity(0.14), in: Capsule())
            .accessibilityElement(children: .combine)
            .accessibilityHint(L10n.serversNotEncryptedHint)
    }
}

/// A server's name and address, with the insecure badge when it applies.
public struct ServerLabel: View {
    let name: String
    let url: String
    let insecure: Bool

    public init(name: String, url: String, insecure: Bool) {
        self.name = name
        self.url = url
        self.insecure = insecure
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
            Text(name)
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(Tokens.Palette.text)
            Text(url)
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.mutedText)
                .lineLimit(1)
                .truncationMode(.middle)
            if insecure {
                InsecureBadge()
            }
        }
        .accessibilityElement(children: .combine)
    }
}
