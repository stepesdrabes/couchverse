import CouchverseCore
import SwiftUI

/// A failure the core reported, in words, with an optional way to try again.
public struct ProblemBanner: View {
    let problem: Problem
    let retry: (() -> Void)?

    public init(_ problem: Problem, retry: (() -> Void)? = nil) {
        self.problem = problem
        self.retry = retry
    }

    public var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: Tokens.Spacing.md) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Tokens.Palette.danger)
                .accessibilityHidden(true)
            Text(problem.message)
                .typeRole(Tokens.TypeRamp.body)
                .foregroundStyle(Tokens.Palette.text)
                .frame(maxWidth: .infinity, alignment: .leading)
                .fixedSize(horizontal: false, vertical: true)
            if let retry {
                Button(L10n.commonRetry, action: retry)
                    .secondaryAction()
            }
        }
        .padding(Tokens.Spacing.lg)
        .background(
            Tokens.Palette.danger.opacity(0.12),
            in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
        )
        .accessibilityElement(children: .combine)
    }
}
