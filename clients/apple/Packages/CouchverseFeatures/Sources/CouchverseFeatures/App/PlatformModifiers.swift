import SwiftUI

extension View {
    /// The remote's Back button on TV; touch devices go back through navigation instead.
    func onBack(perform action: (() -> Void)?) -> some View {
        #if os(tvOS)
            onExitCommand(perform: action)
        #else
            self
        #endif
    }

    /// Groups controls for the TV's focus engine, so moving across columns lands in the group.
    func tvFocusSection() -> some View {
        #if os(tvOS)
            focusSection()
        #else
            self
        #endif
    }

    /// A light tick when the user picks something; TVs have no haptics.
    func selectionHaptic<T: Equatable>(trigger: T) -> some View {
        #if os(iOS)
            sensoryFeedback(.selection, trigger: trigger)
        #else
            self
        #endif
    }

    /// A flow on top of the current screen: a sheet on touch devices, full screen on TV, where a
    /// sheet would leave the screen behind it focusable.
    func modal<Item: Identifiable, Content: View>(
        item: Binding<Item?>, @ViewBuilder content: @escaping (Item) -> Content
    ) -> some View {
        #if os(tvOS)
            fullScreenCover(item: item, content: content)
        #else
            sheet(item: item, content: content)
        #endif
    }
}

extension View {
    func modal<Content: View>(isPresented: Binding<Bool>, @ViewBuilder content: @escaping () -> Content) -> some View {
        #if os(tvOS)
            fullScreenCover(isPresented: isPresented, content: content)
        #else
            sheet(isPresented: isPresented, content: content)
        #endif
    }
}

/// Something a screen asks the root to present. Compared by name: the root's closures only set
/// its own state, so a new closure for the same action changes nothing a screen depends on.
struct RootAction: Equatable {
    let name: String
    let perform: () -> Void

    func callAsFunction() { perform() }

    static func == (lhs: RootAction, rhs: RootAction) -> Bool { lhs.name == rhs.name }
}

extension EnvironmentValues {
    /// Opens "Who's watching?" over the running app (the TV's way to switch profiles).
    @Entry var showProfilePicker = RootAction(name: "none") {}
    /// Opens the account switcher sheet (the phone's way).
    @Entry var showAccountSwitcher = RootAction(name: "none") {}
    /// The date relative times are measured from; `nil` means now (snapshots pin it).
    @Entry var referenceDate: Date?
}
