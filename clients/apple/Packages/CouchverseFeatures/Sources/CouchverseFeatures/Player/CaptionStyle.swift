import MediaAccessibility
import SwiftUI

/// How the viewer likes captions drawn (Accessibility > Subtitles & Captioning > Style), which the
/// system's own captions follow. A sidecar file's lines are drawn by the app, so they follow it
/// too: the text's size against the default, its colour and the colour behind it.
struct CaptionStyle {
    var scale: CGFloat
    var text: Color
    var background: Color

    /// Posted when the viewer changes the style.
    static let changed = Notification.Name(kMACaptionAppearanceSettingsChangedNotification as String)

    /// The viewer's style as the system has it now.
    static func current() -> CaptionStyle {
        let scale = MACaptionAppearanceGetRelativeCharacterSize(.user, nil)
        let text = MACaptionAppearanceCopyForegroundColor(.user, nil).takeRetainedValue()
        let background = MACaptionAppearanceCopyBackgroundColor(.user, nil).takeRetainedValue()
        return CaptionStyle(
            scale: scale > 0 ? scale : 1,
            text: Color(cgColor: text).opacity(MACaptionAppearanceGetForegroundOpacity(.user, nil)),
            background: Color(cgColor: background).opacity(MACaptionAppearanceGetBackgroundOpacity(.user, nil)))
    }
}
