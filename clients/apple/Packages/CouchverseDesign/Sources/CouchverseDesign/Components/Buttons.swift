import SwiftUI

extension View {
    /// The one action a screen leads with: prominent Liquid Glass in the accent, with the core's
    /// contrast-checked text colour on top.
    public func primaryAction() -> some View {
        modifier(PrimaryAction())
    }

    /// Any other action: plain glass, so the primary one stays the only coloured control.
    public func secondaryAction() -> some View {
        modifier(SecondaryAction())
    }
}

private struct SecondaryAction: ViewModifier {
    @Environment(\.ambience) private var ambience

    func body(content: Content) -> some View {
        #if os(tvOS)
            styled(content)
        #else
            styled(content)
                .controlSize(.large)
                .foregroundStyle(Tokens.Palette.text)
        #endif
    }

    @ViewBuilder private func styled(_ content: Content) -> some View {
        if ambience == .flat {
            content.buttonStyle(.bordered)
        } else {
            content.buttonStyle(.glass)
        }
    }
}

private struct PrimaryAction: ViewModifier {
    @Environment(\.accent) private var accent
    @Environment(\.isEnabled) private var isEnabled
    @Environment(\.ambience) private var ambience

    func body(content: Content) -> some View {
        #if os(tvOS)
            // focus turns the platter light and the system picks a dark label; forcing the
            // on-accent colour would leave white on white
            styled(content)
                .tint(accent.color)
                .opacity(isEnabled ? 1 : 0.45)
        #else
            styled(content)
                .controlSize(.large)
                .tint(accent.color)
                .foregroundStyle(accent.onAccent)
                .opacity(isEnabled ? 1 : 0.55)
        #endif
    }

    @ViewBuilder private func styled(_ content: Content) -> some View {
        if ambience == .flat {
            content.buttonStyle(.borderedProminent)
        } else {
            content.buttonStyle(.glassProminent)
        }
    }
}

/// A button label that keeps its width while it shows progress instead of the title.
public struct ActionLabel: View {
    let title: String
    let systemImage: String?
    let busy: Bool

    public init(_ title: String, systemImage: String? = nil, busy: Bool = false) {
        self.title = title
        self.systemImage = systemImage
        self.busy = busy
    }

    public var body: some View {
        ZStack {
            Group {
                if let systemImage {
                    Label(title, systemImage: systemImage)
                } else {
                    Text(title)
                }
            }
            .opacity(busy ? 0 : 1)
            if busy {
                ProgressView()
            }
        }
        .frame(maxWidth: .infinity)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(title)
        .accessibilityValue(busy ? Text(L10n.commonProcessing) : Text(""))
    }
}
