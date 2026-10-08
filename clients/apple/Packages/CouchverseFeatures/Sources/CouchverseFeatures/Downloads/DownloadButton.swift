import CouchverseCore
import CouchverseDesign
import SwiftUI

#if os(iOS)
    /// Keeping a movie or an episode on the device: a menu of qualities until it is asked for,
    /// then where it stands (waiting for the server, preparing, coming down with its progress,
    /// downloaded, failed), with a way to try again or to remove it. Nothing while the server
    /// has downloads turned off and the device keeps none of this one.
    struct DownloadButton: View {
        let target: PlayTarget
        /// Among the title's labelled actions; an icon beside an episode otherwise.
        var labelled = false
        @Environment(CoreRuntime.self) private var core

        private var item: DownloadItem? {
            core.downloads.items.first { $0.target == target }
        }

        var body: some View {
            if let item {
                Menu {
                    Section(DownloadLabels.state(item)) {
                        if item.state == .failed {
                            Button(L10n.downloadRetry, systemImage: "arrow.clockwise") {
                                core.send(.downloadRetried(DownloadRef(id: item.id)))
                            }
                        }
                        Button(L10n.downloadRemove, systemImage: "trash", role: .destructive) {
                            core.send(.downloadRemoved(DownloadRef(id: item.id)))
                        }
                    }
                } label: {
                    DownloadStateLabel(item: item, labelled: labelled)
                }
                .downloadControl(labelled: labelled)
                .accessibilityLabel(L10n.downloadAction)
                .accessibilityValue(DownloadLabels.state(item))
            } else if core.session.features.downloads {
                Menu {
                    Section(L10n.downloadChooseQuality) {
                        ForEach(DownloadLabels.qualities, id: \.self) { quality in
                            Button {
                                core.send(.downloadRequested(DownloadAsk(target: target, quality: quality)))
                            } label: {
                                Text(DownloadLabels.quality(quality))
                                if quality == .original {
                                    Text(L10n.downloadQualityOriginalHint)
                                }
                            }
                        }
                    }
                } label: {
                    if labelled {
                        Label(L10n.downloadAction, systemImage: "arrow.down.circle")
                    } else {
                        Image(systemName: "arrow.down.circle")
                            .font(.title3)
                    }
                }
                .downloadControl(labelled: labelled)
                .accessibilityLabel(L10n.downloadAction)
            }
        }
    }

    /// A download's state as the control's label: a progress ring on its way, a check once it is
    /// on the device, a warning when it failed.
    private struct DownloadStateLabel: View {
        let item: DownloadItem
        let labelled: Bool

        var body: some View {
            if labelled {
                Label {
                    Text(title).monospacedDigit()
                } icon: {
                    icon
                }
            } else {
                icon.font(.title3)
            }
        }

        private var title: String {
            switch item.state {
            case .ready: L10n.downloadStateReady
            case .failed: L10n.commonFailed
            case .queued, .preparing, .fetching: DownloadLabels.state(item)
            }
        }

        @ViewBuilder private var icon: some View {
            switch item.state {
            case .ready:
                Image(systemName: "checkmark.circle.fill").foregroundStyle(Tokens.Palette.success)
            case .failed:
                Image(systemName: "exclamationmark.arrow.circlepath").foregroundStyle(Tokens.Palette.danger)
            case .queued, .preparing, .fetching:
                DownloadRing(item: item)
            }
        }
    }

    extension View {
        /// A labelled action beside My List, or an icon with a finger-sized target beside an
        /// episode.
        @ViewBuilder func downloadControl(labelled: Bool) -> some View {
            if labelled {
                secondaryAction()
            } else {
                frame(minWidth: 44, minHeight: 44)
                    .contentShape(Rectangle())
                    .buttonStyle(.borderless)
            }
        }
    }
#endif
