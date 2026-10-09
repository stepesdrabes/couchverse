import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The design system's components on one canvas, for a snapshot that covers what the screens
/// draw flat (the glow backdrop) or rarely (markdown, every badge and banner).
struct DesignGallery: View {
    @State private var text = "media.example.com"

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                HStack(spacing: Tokens.Spacing.md) {
                    LogoMark(size: 56)
                    Text("Couchverse").typeRole(Tokens.TypeRamp.hero)
                }
                HStack(spacing: Tokens.Spacing.lg) {
                    AvatarView(url: nil, seed: "nora", name: "Nora").frame(width: 64, height: 64)
                    AvatarView(url: nil, seed: "admin", name: "Admin").frame(width: 64, height: 64)
                    Skeleton(circle: 64)
                    InsecureBadge()
                }
                ServerLabel(name: "Home Media", url: "http://192.168.1.5:8080", insecure: true)
                ProblemBanner(Problem(code: "timeout", detail: "")) {}
                FormField(L10n.serversAddressLabel, text: $text, kind: .address)
                HStack(spacing: Tokens.Spacing.md) {
                    Button {
                    } label: {
                        ActionLabel(L10n.pairingApprove, systemImage: "checkmark")
                    }
                    .primaryAction()
                    Button {
                    } label: {
                        ActionLabel(L10n.pairingDeny)
                    }
                    .secondaryAction()
                }
                HStack(alignment: .top, spacing: Tokens.Spacing.xl) {
                    QRCodeView("http://192.168.1.5:8080/pair?code=WDJB-MJHT").frame(width: 140)
                    MarkdownView(Self.bio)
                }
            }
            .padding(Tokens.Spacing.xl)
        }
        .background { GlowBackdrop(tint: Tokens.Palette.accent) }
    }

    static let bio = MarkdownDoc(blocks: [
        .heading(HeadingBlock(level: 2, inlines: [.text("Film nights")])),
        .paragraph([
            .text("Mostly "), .strong([.text("sci-fi")]), .text(" and "), .emphasis([.text("slow cinema")]),
            .text(", see "), .link(LinkInline(href: "https://example.com", children: [.text("my list")])),
        ]),
        .list(
            ListBlock(
                start: nil,
                items: [
                    ListItem(blocks: [.paragraph([.text("Arrival")])]),
                    ListItem(blocks: [.paragraph([.text("Stalker")])]),
                ])),
        .quote([.paragraph([.text("Cinema is a matter of what's in the frame.")])]),
        .code("couchverse://pair"),
    ])
}
