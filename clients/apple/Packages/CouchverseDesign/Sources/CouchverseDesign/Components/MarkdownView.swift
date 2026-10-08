import CouchverseCore
import SwiftUI

/// User markdown (bios) rendered from the core's safe document tree (D19). The tree can only
/// express text, emphasis, lists, quotes, code, tables and http(s)/mailto links, so nothing
/// here interprets raw markup.
public struct MarkdownView: View {
    let doc: MarkdownDoc

    public init(_ doc: MarkdownDoc) {
        self.doc = doc
    }

    public var body: some View {
        BlockStack(blocks: doc.blocks)
    }
}

private struct BlockStack: View {
    let blocks: [Block]

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            ForEach(Array(blocks.enumerated()), id: \.offset) { _, block in
                BlockView(block: block)
            }
        }
    }
}

private struct BlockView: View {
    let block: Block
    @Environment(\.accent) private var accent

    var body: some View {
        switch block {
        case .paragraph(let inlines):
            Text(Inlines.attributed(inlines))
                .typeRole(Tokens.TypeRamp.body)
                .fixedSize(horizontal: false, vertical: true)
        case .heading(let heading):
            Text(Inlines.attributed(heading.inlines))
                .typeRole(heading.level <= 2 ? Tokens.TypeRamp.section : Tokens.TypeRamp.card)
                .accessibilityAddTraits(.isHeader)
        case .quote(let blocks):
            HStack(alignment: .top, spacing: Tokens.Spacing.md) {
                Capsule().fill(accent.color).frame(width: 3)
                AnyView(BlockStack(blocks: blocks)).foregroundStyle(Tokens.Palette.mutedText)
            }
        case .list(let list):
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                ForEach(Array(list.items.enumerated()), id: \.offset) { index, item in
                    HStack(alignment: .firstTextBaseline, spacing: Tokens.Spacing.sm) {
                        Text(list.start.map { "\(Int($0) + index)." } ?? "\u{2022}")
                            .foregroundStyle(Tokens.Palette.mutedText)
                            .monospacedDigit()
                        AnyView(BlockStack(blocks: item.blocks))
                    }
                }
            }
        case .code(let code):
            Text(code)
                .font(.system(.callout, design: .monospaced))
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(Tokens.Spacing.md)
                .background(
                    Tokens.Palette.surface2,
                    in: RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous))
        case .rule:
            Divider().overlay(Tokens.Palette.edgeLine)
        case .table(let table):
            Grid(alignment: .leading, horizontalSpacing: Tokens.Spacing.lg, verticalSpacing: Tokens.Spacing.sm) {
                GridRow {
                    ForEach(Array(table.header.enumerated()), id: \.offset) { _, cell in
                        Text(Inlines.attributed(cell.inlines)).bold()
                    }
                }
                Divider().overlay(Tokens.Palette.edgeLine)
                ForEach(Array(table.rows.enumerated()), id: \.offset) { _, row in
                    GridRow {
                        ForEach(Array(row.cells.enumerated()), id: \.offset) { _, cell in
                            Text(Inlines.attributed(cell.inlines))
                        }
                    }
                }
            }
            .typeRole(Tokens.TypeRamp.body)
        }
    }
}

enum Inlines {
    static func attributed(_ inlines: [Inline], _ intent: InlinePresentationIntent = []) -> AttributedString {
        inlines.reduce(into: AttributedString()) { result, inline in
            switch inline {
            case .text(let text):
                result += styled(text, intent)
            case .code(let code):
                result += styled(code, intent.union(.code))
            case .strong(let children):
                result += attributed(children, intent.union(.stronglyEmphasized))
            case .emphasis(let children):
                result += attributed(children, intent.union(.emphasized))
            case .strike(let children):
                result += attributed(children, intent.union(.strikethrough))
            case .link(let link):
                var text = attributed(link.children, intent)
                #if os(tvOS)
                    // a TV has no browser to open it in
                    text.underlineStyle = .single
                #else
                    text.link = URL(string: link.href)
                #endif
                result += text
            case .break:
                result += AttributedString("\n")
            }
        }
    }

    private static func styled(_ text: String, _ intent: InlinePresentationIntent) -> AttributedString {
        var string = AttributedString(text)
        if !intent.isEmpty {
            string.inlinePresentationIntent = intent
        }
        return string
    }
}
