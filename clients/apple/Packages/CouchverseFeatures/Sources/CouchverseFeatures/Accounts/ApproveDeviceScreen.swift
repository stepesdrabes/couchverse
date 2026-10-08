import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Approving another device's sign-in (a TV showing a pairing code) as the signed-in account: by
/// typing its code, scanning its QR code, or from a `couchverse://pair` link.
struct ApproveDeviceScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.dismiss) private var dismiss
    @State private var code = ""
    @State private var deviceName = ""
    /// The core holds an approval for this presentation (from the field, a scan or a link).
    @State private var opened: Bool
    @State private var scanning = false
    @State private var decided = 0

    init(openedFromLink: Bool = false) {
        _opened = State(initialValue: openedFromLink)
    }

    private var approval: PairingApprovalView { core.pairingApproval }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                    if opened {
                        review
                    } else {
                        codeEntry
                    }
                }
                .readableWidth(Idiom.isTV ? 900 : 520)
                .padding(Tokens.Spacing.xl)
                .motion(Tokens.Motion.smooth, value: approval)
            }
            .scrollBounceBehavior(.basedOnSize)
            .navigationTitle(L10n.pairingApproveTitle)
            .navigationBarTitleDisplayModeInline()
            .toolbar {
                if !Idiom.isTV {
                    ToolbarItem(placement: .cancellationAction) {
                        Button(approval.outcome == nil ? L10n.commonCancel : L10n.commonDone) { dismiss() }
                    }
                }
            }
        }
        .selectionHaptic(trigger: decided)
        .onChange(of: approval.deviceName, initial: true) { _, name in
            if deviceName.isEmpty {
                deviceName = name
            }
        }
        #if os(iOS)
            .sheet(isPresented: $scanning) {
                QRScannerSheet(expecting: .approve) { url in
                    scanning = false
                    open { core.send(.linkOpened(Link(url: url))) }
                }
            }
        #endif
    }

    @ViewBuilder private var codeEntry: some View {
        Text(L10n.accountsPairingInstructions)
            .typeRole(Tokens.TypeRamp.body)
            .foregroundStyle(Tokens.Palette.mutedText)
        FormField(L10n.pairingCodeLabel, text: $code, prompt: "XXXX-XXXX", kind: .code)
            .onChange(of: code) { _, typed in
                let formatted = UserCodeInput.format(typed)
                if formatted != typed {
                    code = formatted
                }
            }
            .submitLabel(.continue)
            .onSubmit(submitCode)
            .accessibilityIdentifier("pairing-code-field")
        Button(action: submitCode) {
            ActionLabel(L10n.pairingContinue)
        }
        .primaryAction()
        .disabled(!UserCodeInput.isComplete(code))
        #if os(iOS)
            Button {
                scanning = true
            } label: {
                ActionLabel(L10n.pairingScanTv, systemImage: "qrcode.viewfinder")
            }
            .secondaryAction()
        #endif
    }

    @ViewBuilder private var review: some View {
        switch approval.status {
        case .loading where approval.deviceName.isEmpty, .idle:
            HStack(spacing: Tokens.Spacing.md) {
                ProgressView()
                Text(approval.code)
                    .font(.system(.title3, design: .monospaced, weight: .semibold))
            }
            .foregroundStyle(Tokens.Palette.mutedText)
        case .notFound:
            outcome(L10n.pairingNotFound, systemImage: "questionmark.circle.fill", tint: Tokens.Palette.danger)
            tryAnotherCode
        case .failed:
            if approval.problem?.code == "unauthorized" {
                outcome(
                    L10n.pairingSignInFirst, systemImage: "person.crop.circle.badge.exclamationmark",
                    tint: Tokens.Palette.danger)
            } else if let problem = approval.problem {
                ProblemBanner(problem) { open { core.send(.pairingApprovalOpened(UserCode(code: approval.code))) } }
            }
            tryAnotherCode
        default:
            if let result = approval.outcome {
                let approved = result == .approved
                let name = deviceName.isEmpty ? approval.deviceName : deviceName
                outcome(
                    approved ? L10n.pairingApproved(device: name) : L10n.pairingDenied(device: name),
                    systemImage: approved ? "checkmark.circle.fill" : "xmark.circle.fill",
                    tint: approved ? Tokens.Palette.success : Tokens.Palette.mutedText)
                Button(L10n.commonDone) { dismiss() }
                    .primaryAction()
            } else {
                request
            }
        }
    }

    @ViewBuilder private var request: some View {
        let busy = approval.status == .loading
        HStack(alignment: .center, spacing: Tokens.Spacing.lg) {
            Image(systemName: DevicePlatform.symbol(approval.platform))
                .font(.system(size: 44))
                .foregroundStyle(Tokens.Palette.text)
                .frame(width: 72, height: 72)
                .glassEffect(.regular, in: RoundedRectangle(cornerRadius: Tokens.Radius.card))
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                Text(
                    L10n.pairingRequest(
                        device: approval.deviceName, name: core.session.user?.displayName ?? "")
                )
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(Tokens.Palette.text)
                .fixedSize(horizontal: false, vertical: true)
                Text("\(DevicePlatform.name(approval.platform)) \u{00B7} \(approval.code)")
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
            }
        }
        .accessibilityElement(children: .combine)
        FormField(L10n.pairingDeviceNameLabel, text: $deviceName, kind: .name)
            .disabled(busy)
        HStack(spacing: Tokens.Spacing.md) {
            Button {
                decided += 1
                core.send(.pairingDenied(UserCode(code: approval.code)))
            } label: {
                ActionLabel(L10n.pairingDeny)
            }
            .secondaryAction()
            Button {
                decided += 1
                core.send(.pairingApproved(PairingApproval(code: approval.code, deviceName: deviceName)))
            } label: {
                ActionLabel(L10n.pairingApprove, systemImage: "checkmark", busy: busy)
            }
            .primaryAction()
            .accessibilityIdentifier("approve-pairing")
        }
        .disabled(busy)
    }

    private var tryAnotherCode: some View {
        Button(L10n.pairingTryAnother) {
            opened = false
            code = ""
        }
        .secondaryAction()
    }

    private func outcome(_ text: String, systemImage: String, tint: Color) -> some View {
        Label {
            Text(text)
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.text)
                .fixedSize(horizontal: false, vertical: true)
        } icon: {
            Image(systemName: systemImage).foregroundStyle(tint)
        }
        .font(.title2)
    }

    private func submitCode() {
        guard UserCodeInput.isComplete(code) else { return }
        open { core.send(.pairingApprovalOpened(UserCode(code: code))) }
    }

    private func open(_ send: () -> Void) {
        deviceName = ""
        send()
        opened = true
    }
}

/// A pairing user code as typed: letters only (the server's alphabet has no digits or vowels to
/// mistake), upper case, grouped `XXXX-XXXX`.
enum UserCodeInput {
    static func format(_ typed: String) -> String {
        let letters = typed.uppercased().filter { $0.isASCII && $0.isLetter }.prefix(8)
        guard letters.count > 4 else { return String(letters) }
        return "\(letters.prefix(4))-\(letters.dropFirst(4))"
    }

    static func isComplete(_ code: String) -> Bool {
        format(code).count == 9
    }
}

/// How a device's platform (as the API names it) is shown.
enum DevicePlatform {
    static func symbol(_ platform: String) -> String {
        switch platform {
        case "ios": "iphone"
        case "ipados": "ipad"
        case "tvos": "appletv"
        case "android": "smartphone"
        case "androidtv": "tv"
        default: "globe"
        }
    }

    static func name(_ platform: String) -> String {
        switch platform {
        case "ios": L10n.devicesPlatformIos
        case "ipados": L10n.devicesPlatformIpados
        case "tvos": L10n.devicesPlatformTvos
        case "android": L10n.devicesPlatformAndroid
        case "androidtv": L10n.devicesPlatformAndroidtv
        default: L10n.devicesPlatformWeb
        }
    }
}
