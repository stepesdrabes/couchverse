import CouchverseCore
import CouchverseDesign
import CouchverseShared
import Foundation

#if os(iOS)
    import ActivityKit
    import os
#endif

extension CouchActivityContent {
    /// The Live Activity of this device's couch, in the display language; `nil` while it is on none
    /// or the session has no code yet.
    static func make(couch: CouchView, player: PlayerView, accent: String) -> CouchActivityContent? {
        guard couch.isLive, let code = couch.code else { return nil }
        // the title is known once this device plays what the couch watches; a remote never does
        let watching = player.target != nil && player.target == couch.media
        let members = couch.members.filter(\.host) + couch.members.filter { !$0.host }
        let spaced = CouchLabels.spaced(code)
        return CouchActivityContent(
            heading: L10n.couchOpen,
            title: watching ? player.title : nil,
            detail: watching && !player.subtitle.isEmpty ? player.subtitle : nil,
            members: members.prefix(4).map(\.displayName),
            membersLine: L10n.couchOnCouchCount(count: String(couch.members.count)),
            count: couch.members.count,
            code: spaced,
            codeLine: L10n.couchCode(code: spaced),
            status: CouchLabels.status(couch),
            staleNote: L10n.widgetCouchStale,
            accent: accent)
    }
}

#if os(iOS)
    nonisolated private let log = Logger(subsystem: "io.stepes.couchverse", category: "live-activity")

    /// The couch Live Activity (plan 10.9), drawn by the widget extension: started once this device
    /// is on a couch, updated by the app while it runs (playing keeps it running) and ended with the
    /// session. Updates are local, as pushing them needs a paid team, so each one holds for a few
    /// minutes and the activity shows it is out of date once the app is suspended.
    @MainActor
    final class CouchActivities {
        /// How long an update holds; the app renews it every minute while it runs.
        static let freshFor: TimeInterval = 180
        /// The activity shown, by its id, and the session it is for.
        private var shown: (id: String, session: String)?
        private var wanted: (content: CouchActivityContent, session: String)?
        private var work: Task<Void, Never>?
        private var cleared = false

        /// Shows `content` for the session `session` (again, which renews it), or ends it.
        func show(_ content: CouchActivityContent?, session: String?) {
            if let content, let session {
                wanted = (content, session)
            } else {
                wanted = nil
            }
            let before = work
            work = Task {
                await before?.value
                await apply()
            }
        }

        private func apply() async {
            if !cleared {
                cleared = true
                // the couch lives in the app's memory, so whatever an earlier launch left is over
                await Self.end(nil)
            }
            guard let wanted else {
                await end()
                return
            }
            if let shown, shown.session != wanted.session {
                await end()
            }
            let content = ActivityContent(
                state: wanted.content, staleDate: Date.now.addingTimeInterval(Self.freshFor))
            if let shown {
                await Self.update(shown.id, content)
            } else if ActivityAuthorizationInfo().areActivitiesEnabled {
                do {
                    let activity = try Activity.request(
                        attributes: CouchActivityAttributes(session: wanted.session), content: content)
                    shown = (activity.id, wanted.session)
                } catch {
                    log.error("could not start the couch activity: \(error)")
                }
            }
        }

        private func end() async {
            guard let shown else { return }
            self.shown = nil
            await Self.end(shown.id)
        }

        private nonisolated static func update(_ id: String, _ content: ActivityContent<CouchActivityContent>) async {
            await Activity<CouchActivityAttributes>.activities.first { $0.id == id }?.update(content)
        }

        /// Ends the activity `id`, or every one with `nil`.
        private nonisolated static func end(_ id: String?) async {
            for activity in Activity<CouchActivityAttributes>.activities where id == nil || activity.id == id {
                await activity.end(nil, dismissalPolicy: .immediate)
            }
        }
    }
#endif
