import SwiftUI

/// What a text field holds, which decides its keyboard, autofill and capitalisation.
public enum FieldKind {
    case address
    case username
    case password
    case code
    case name
    /// A numeric code, typed on the number pad.
    case digits
}

/// A labelled text field. On TV it is the system field (it opens the full-screen keyboard and
/// takes focus like any control); on touch devices a rounded surface with the label above.
public struct FormField: View {
    let label: String
    let prompt: String?
    let kind: FieldKind
    @Binding var text: String

    public init(_ label: String, text: Binding<String>, prompt: String? = nil, kind: FieldKind) {
        self.label = label
        self.prompt = prompt
        self.kind = kind
        _text = text
    }

    public var body: some View {
        #if os(tvOS)
            field.accessibilityLabel(label)
        #else
            VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                // the field itself carries the label for VoiceOver
                Text(label)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
                    .accessibilityHidden(true)
                field
                    .padding(.horizontal, Tokens.Spacing.md)
                    .padding(.vertical, Tokens.Spacing.md)
                    .background(
                        Tokens.Palette.surface2,
                        in: RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous)
                    )
                    .overlay {
                        RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous)
                            .strokeBorder(Tokens.Palette.edgeLine)
                    }
                    .accessibilityLabel(label)
            }
        #endif
    }

    @ViewBuilder private var field: some View {
        // the label sits above the field on touch devices, so only a TV repeats it inside
        let placeholder = Text(prompt ?? (Idiom.isTV ? label : "")).foregroundStyle(Tokens.Palette.faintText)
        Group {
            if kind == .password {
                SecureField(label, text: $text, prompt: placeholder)
            } else {
                TextField(label, text: $text, prompt: placeholder)
            }
        }
        .textContentType(contentType)
        .autocorrectionDisabled()
        .textInputAutocapitalization(capitalization)
        .keyboardType(keyboard)
        .font(kind == .code || kind == .digits ? .system(.title3, design: .monospaced, weight: .semibold) : .body)
        .foregroundStyle(Tokens.Palette.text)
    }

    private var contentType: UITextContentType? {
        switch kind {
        case .address: .URL
        case .username: .username
        case .password: .password
        case .code, .digits: .oneTimeCode
        case .name: nil
        }
    }

    private var capitalization: TextInputAutocapitalization {
        switch kind {
        case .code: .characters
        case .name: .words
        default: .never
        }
    }

    private var keyboard: UIKeyboardType {
        switch kind {
        case .address: .URL
        case .code: .asciiCapable
        case .digits: .numberPad
        default: .default
        }
    }
}
