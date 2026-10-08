import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The core's transient notices (a My List change the server refused, ...) as toasts, each
/// dismissed after a few seconds or with a tap; the core is told so it forgets it.
struct NoticeToasts: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(spacing: Tokens.Spacing.sm) {
            ForEach(core.notices.notices, id: \.id) { notice in
                Toast(notice: notice) { dismiss(notice) }
                    .transition(
                        reduceMotion ? .opacity : .move(edge: Idiom.isTV ? .top : .bottom).combined(with: .opacity)
                    )
                    .task(id: notice.id) {
                        // gone in a few seconds, so VoiceOver says it at once
                        AccessibilityNotification.Announcement(CatalogLabels.notice(notice.code)).post()
                        try? await Task.sleep(for: .seconds(4))
                        if !Task.isCancelled {
                            dismiss(notice)
                        }
                    }
            }
        }
        .padding(Tokens.Spacing.xl)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: Idiom.isTV ? .top : .bottom)
        .motion(Tokens.Motion.smooth, value: core.notices.notices)
    }

    private func dismiss(_ notice: Notice) {
        core.send(.noticeDismissed(NoticeRef(id: notice.id)))
    }
}

private struct Toast: View {
    let notice: Notice
    let dismiss: () -> Void

    var body: some View {
        Label(CatalogLabels.notice(notice.code), systemImage: "exclamationmark.circle")
            .typeRole(Tokens.TypeRamp.card)
            .foregroundStyle(Tokens.Palette.text)
            .padding(.horizontal, Tokens.Spacing.xl)
            .padding(.vertical, Tokens.Spacing.md)
            .glassEffect(.regular, in: Capsule())
            .onTapGesture(perform: dismiss)
            .accessibilityAddTraits(.isStaticText)
            .accessibilityAction(named: L10n.commonDone, dismiss)
    }
}
