import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Joining a couch session over the app, from a link, Settings or the welcome screen; once the core
/// seats this device the cover moves on to the player or the remote.
struct JoinCouchScreen: View {
    let invite: CouchInvite
    /// Shows the outcome of a join already asked for (snapshots).
    var attempted = false
    let close: () -> Void

    @Environment(\.accent) private var accent

    var body: some View {
        ZStack(alignment: .topLeading) {
            GlowBackdrop(tint: accent.color, intensity: 0.6)
            ScrollView {
                JoinCouchForm(invite: invite, attempted: attempted)
                    .readableWidth(Idiom.isTV ? 1500 : 520)
                    .padding(.horizontal, Idiom.isTV ? 80 : Tokens.Spacing.xl)
                    .padding(.vertical, Idiom.isTV ? 60 : 72)
            }
            .scrollBounceBehavior(.basedOnSize)
            #if os(iOS)
                Button(action: close) {
                    Image(systemName: "xmark")
                        .font(.system(size: 17, weight: .semibold))
                        .frame(width: 44, height: 44)
                }
                .buttonStyle(.glass)
                .buttonBorderShape(.circle)
                .accessibilityLabel(L10n.commonCancel)
                .padding(Tokens.Spacing.lg)
            #endif
        }
    }
}

/// The six-digit code, which a link or a scanned QR code fills in, and the ways to join with it: as
/// a viewer, or on a phone as a remote for this account's own player. A TV types it on a digit pad.
/// Without an account it also takes the server's address (filled in by a link or a scanned join
/// page too) and joins as a guest.
struct JoinCouchForm: View {
    @State private var code: String
    /// The server a link or a scanned join page named, or the one typed without an account.
    @State private var server: String
    /// This form asked to join, so the core's couch view describes its attempt.
    @State private var attempted: Bool
    @State private var scanning = false
    @FocusState private var focus: JoinFocus?
    @Environment(CoreRuntime.self) private var core

    init(invite: CouchInvite = CouchInvite(), attempted: Bool = false) {
        _code = State(initialValue: CouchCodeInput.format(invite.code))
        _server = State(initialValue: invite.server ?? "")
        _attempted = State(initialValue: attempted)
    }

    /// No account to join through: the server is asked for, and the join is a guest's.
    private var guest: Bool { core.app.phase != .ready }
    private var address: String { server.trimmingCharacters(in: .whitespaces) }
    private var joining: Bool { attempted && core.couch.status == .connecting && core.couch.role == nil }
    private var problem: Problem? { attempted && core.couch.status == .idle ? core.couch.problem : nil }
    private var ready: Bool { CouchCodeInput.isComplete(code) && !joining && !(guest && address.isEmpty) }

    var body: some View {
        Group {
            if Idiom.isTV {
                HStack(alignment: .top, spacing: 96) {
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                        header
                        if guest {
                            serverField
                        }
                        CodeSlots(code: code)
                        failure
                        joinButton
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .tvFocusSection()
                    DigitPad(code: $code, focus: $focus)
                        .tvFocusSection()
                }
                .defaultFocus($focus, startingFocus)
                .onChange(of: code) { _, code in
                    if CouchCodeInput.isComplete(code) {
                        focus = guest && address.isEmpty ? .server : .join
                    }
                }
            } else {
                VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
                    header
                    if guest {
                        serverField
                    }
                    FormField(L10n.couchJoinCode, text: $code, prompt: "123456", kind: .digits)
                        .focused($focus, equals: .code)
                        .onChange(of: code) { _, typed in
                            let formatted = CouchCodeInput.format(typed)
                            if formatted != typed {
                                code = formatted
                            }
                        }
                        .submitLabel(.join)
                        .onSubmit { join(remote: false) }
                        .accessibilityIdentifier("couch-code-field")
                    failure
                    joinButton
                    // a remote steers this account's own player, which a guest has none of
                    if !guest {
                        Button {
                            join(remote: true)
                        } label: {
                            ActionLabel(L10n.couchJoinRemote, systemImage: "appletvremote.gen4.fill")
                        }
                        .secondaryAction()
                        .disabled(!ready)
                    }
                    #if os(iOS)
                        Button {
                            scanning = true
                        } label: {
                            ActionLabel(L10n.scannerTitle, systemImage: "qrcode.viewfinder")
                        }
                        .secondaryAction()
                    #endif
                }
            }
        }
        .motion(Tokens.Motion.smooth, value: problem)
        #if os(iOS)
            .sheet(isPresented: $scanning) {
                QRScannerSheet(expecting: .couch) { url in
                    scanning = false
                    if let invite = CouchLink.invite(url) {
                        code = invite.code
                        server = invite.server ?? server
                    }
                }
            }
        #endif
    }

    /// The TV starts on what is still missing: the server, then the code, then joining.
    private var startingFocus: JoinFocus {
        if guest && address.isEmpty {
            return .server
        }
        return CouchCodeInput.isComplete(code) ? .join : .digit("1")
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            Image(systemName: "sofa.fill")
                .font(.system(size: Idiom.isTV ? 64 : 40, weight: .semibold))
                .foregroundStyle(.tint)
                .accessibilityHidden(true)
            Text(L10n.couchJoinTitle)
                .typeRole(Tokens.TypeRamp.title)
                .foregroundStyle(Tokens.Palette.text)
                .accessibilityAddTraits(.isHeader)
            Text(guest ? L10n.couchJoinGuestHint : L10n.couchJoinHint)
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.mutedText)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var serverField: some View {
        FormField(
            L10n.serversAddressLabel, text: $server, prompt: L10n.serversAddressPlaceholder, kind: .address
        )
        .focused($focus, equals: .server)
        .submitLabel(.next)
        .onSubmit { focus = Idiom.isTV ? .digit("1") : .code }
        .accessibilityIdentifier("couch-server-field")
    }

    @ViewBuilder private var failure: some View {
        if let problem {
            ProblemBanner(problem)
        }
    }

    private var joinButton: some View {
        Button {
            join(remote: false)
        } label: {
            ActionLabel(joining ? L10n.couchConnecting : L10n.couchJoin, systemImage: "play.fill", busy: joining)
        }
        .primaryAction()
        .disabled(!ready)
        .focused($focus, equals: .join)
        .accessibilityIdentifier("couch-join")
    }

    private func join(remote: Bool) {
        guard ready else { return }
        attempted = true
        // the core joins through the account on its own server, and as a guest anywhere else
        let request = CouchCode(code: code, server: address.isEmpty ? nil : address)
        core.send(remote ? .couchRemoteRequested(CouchCode(code: code)) : .couchJoinRequested(request))
    }
}

enum JoinFocus: Hashable {
    case server
    case code
    case join
    case digit(String)
}

/// The code on TV, six boxes filling up as the digit pad types.
private struct CodeSlots: View {
    let code: String
    @Environment(\.accent) private var accent

    var body: some View {
        let digits = Array(code)
        HStack(spacing: Tokens.Spacing.md) {
            ForEach(0..<6, id: \.self) { index in
                Text(index < digits.count ? String(digits[index]) : " ")
                    .font(.system(size: 64, weight: .bold, design: .monospaced))
                    .foregroundStyle(Tokens.Palette.text)
                    .frame(width: 88, height: 112)
                    .background(
                        Tokens.Palette.surface2,
                        in: RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous)
                    )
                    .overlay {
                        RoundedRectangle(cornerRadius: Tokens.Radius.input, style: .continuous)
                            .strokeBorder(index == digits.count ? accent.ink : Tokens.Palette.edgeLine, lineWidth: 2)
                    }
                    .padding(.leading, index == 3 ? Tokens.Spacing.lg : 0)
            }
        }
        .accessibilityElement()
        .accessibilityLabel(L10n.couchJoinCode)
        .accessibilityValue(code)
    }
}

/// Digits in phone order, then zero and delete: typing with the remote's arrows and select.
private struct DigitPad: View {
    @Binding var code: String
    var focus: FocusState<JoinFocus?>.Binding

    var body: some View {
        Grid(horizontalSpacing: 28, verticalSpacing: 28) {
            ForEach([["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"]], id: \.self) { row in
                GridRow {
                    ForEach(row, id: \.self, content: key)
                }
            }
            GridRow {
                Color.clear.frame(width: 1, height: 1)
                key("0")
                Button {
                    code = String(code.dropLast())
                } label: {
                    Image(systemName: "delete.left")
                        .font(.system(size: 40, weight: .semibold))
                        .frame(width: 120, height: 96)
                }
                .buttonStyle(.bordered)
                .focused(focus, equals: .digit("delete"))
                .accessibilityLabel(L10n.commonDelete)
            }
        }
    }

    private func key(_ digit: String) -> some View {
        Button {
            code = CouchCodeInput.format(code + digit)
        } label: {
            Text(digit)
                .font(.system(size: 48, weight: .semibold, design: .rounded))
                .frame(width: 120, height: 96)
        }
        .buttonStyle(.bordered)
        .focused(focus, equals: .digit(digit))
    }
}
