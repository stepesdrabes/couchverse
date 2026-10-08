import CouchverseDesign
import SwiftUI

extension EnvironmentValues {
    /// What a tab's cards and the titles they open zoom between on iPhone and iPad (plan 10.3);
    /// nothing on TV, where a card lifts with the focus instead.
    @Entry var cardZoom: Namespace.ID?
}

extension View {
    /// A card a title opens from, which the title zooms out of and back into. `id` names this
    /// card among every card on screen: a title on two shelves is two cards.
    func zoomSource(_ id: String) -> some View {
        modifier(ZoomSource(id: id))
    }

    /// A title zooms out of the card `id` names; opened from elsewhere (a link, a button) it is
    /// pushed as usual.
    func zoomed(from id: String?) -> some View {
        modifier(Zoomed(id: id))
    }
}

private struct ZoomSource: ViewModifier {
    let id: String
    @Environment(\.cardZoom) private var namespace

    func body(content: Content) -> some View {
        if let namespace {
            content.matchedTransitionSource(id: id, in: namespace) { source in
                source.clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            }
        } else {
            content
        }
    }
}

private struct Zoomed: ViewModifier {
    let id: String?
    @Environment(\.cardZoom) private var namespace

    func body(content: Content) -> some View {
        if let id, let namespace {
            content.navigationTransition(.zoom(sourceID: id, in: namespace))
        } else {
            content
        }
    }
}
