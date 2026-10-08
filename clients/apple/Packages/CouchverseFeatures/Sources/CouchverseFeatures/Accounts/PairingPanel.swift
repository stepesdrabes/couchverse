import CouchverseCore
import CouchverseDesign
import SwiftUI

/// "Sign in with another device": the code shown large, the QR code of the server's pairing page
/// for a phone to scan, and how long the code still works.
struct PairingPanel: View {
    let pairing: PairingView?
    let loading: Bool
    let problem: Problem?
    let onNewCode: () -> Void

    @Environment(CoreRuntime.self) private var core
    @ScaledMetric(relativeTo: .largeTitle) private var codeSize: CGFloat = Idiom.isTV ? 92 : 40
    @ScaledMetric(relativeTo: .body) private var qrSize: CGFloat = Idiom.isTV ? 340 : 200

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
            Text(L10n.accountsSignInWithDevice)
                .typeRole(Tokens.TypeRamp.section)
                .foregroundStyle(Tokens.Palette.text)
                .accessibilityAddTraits(.isHeader)
            if let pairing {
                ViewThatFits(in: .horizontal) {
                    HStack(alignment: .center, spacing: Tokens.Spacing.xxl) { content(pairing) }
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xl) { content(pairing) }
                }
            } else if loading {
                HStack(alignment: .center, spacing: Tokens.Spacing.xxl) {
                    Skeleton(width: qrSize, height: qrSize, cornerRadius: Tokens.Radius.card)
                    VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                        Skeleton(width: codeSize * 5, height: codeSize)
                        Skeleton(width: codeSize * 4, height: 18)
                    }
                }
            } else if let problem {
                ProblemBanner(problem, retry: onNewCode)
            } else {
                // a password attempt ends any pairing; offer a fresh code
                Button(action: onNewCode) {
                    ActionLabel(L10n.accountsPairingNewCode, systemImage: "qrcode")
                }
                .secondaryAction()
                .fixedSize()
            }
        }
    }

    @ViewBuilder private func content(_ pairing: PairingView) -> some View {
        let waiting = pairing.state == .waiting
        QRCodeView(pairing.verifyUrl)
            .frame(width: qrSize, height: qrSize)
            .opacity(waiting ? 1 : 0.2)
            .motion(Tokens.Motion.smooth, value: waiting)
        VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
            Text(L10n.accountsPairingInstructions)
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.mutedText)
                .fixedSize(horizontal: false, vertical: true)
            Text(pairing.userCode)
                .font(.system(size: codeSize, weight: .bold, design: .monospaced))
                .tracking(codeSize * 0.08)
                .foregroundStyle(waiting ? Tokens.Palette.text : Tokens.Palette.faintText)
                .strikethrough(!waiting, color: Tokens.Palette.faintText)
                .minimumScaleFactor(0.5)
                .lineLimit(1)
                .speechSpellsOutCharacters()
                .accessibilityIdentifier("pairing-code")
            Text(L10n.accountsPairingVisit(url: Self.displayed(pairing.verifyUrl)))
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.mutedText)
            status(pairing)
        }
    }

    @ViewBuilder private func status(_ pairing: PairingView) -> some View {
        switch pairing.state {
        case .waiting:
            TimelineView(.periodic(from: .now, by: 1)) { _ in
                HStack(spacing: Tokens.Spacing.md) {
                    ProgressView()
                    Text(L10n.accountsPairingExpiresIn(time: Self.countdown(pairing.expiresAtMs, now: core.nowMs())))
                        .monospacedDigit()
                }
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.mutedText)
                .accessibilityElement(children: .combine)
            }
        case .expired, .denied:
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                Label(
                    pairing.state == .expired ? L10n.accountsPairingExpired : L10n.accountsPairingDenied,
                    systemImage: pairing.state == .expired ? "clock.badge.exclamationmark" : "hand.raised.fill"
                )
                .foregroundStyle(Tokens.Palette.danger)
                Button(action: onNewCode) {
                    ActionLabel(L10n.accountsPairingNewCode, systemImage: "arrow.clockwise")
                }
                .primaryAction()
                .fixedSize()
            }
        }
    }

    /// `m:ss` until the deadline, on the core's clock.
    static func countdown(_ deadline: UInt64, now: UInt64) -> String {
        let seconds = Int((deadline > now ? deadline - now : 0) / 1000)
        return Duration.seconds(seconds).formatted(.time(pattern: .minuteSecond))
    }

    /// The pairing page without its scheme, for reading off the screen.
    static func displayed(_ url: String) -> String {
        url.replacingOccurrences(of: "https://", with: "").replacingOccurrences(of: "http://", with: "")
    }
}
