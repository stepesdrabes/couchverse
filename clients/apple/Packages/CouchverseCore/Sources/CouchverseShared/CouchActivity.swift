import Foundation

#if os(iOS)
    import ActivityKit
#endif

/// What the couch Live Activity shows, made by the app with its words in the display language and
/// drawn by the widget extension.
public struct CouchActivityContent: Codable, Sendable, Hashable {
    /// "Couch session".
    public var heading: String
    /// What is on, once this device plays it.
    public var title: String?
    /// The episode's label.
    public var detail: String?
    /// The members' names, the host first; a few at most.
    public var members: [String]
    /// "On the couch · 3".
    public var membersLine: String
    /// How many are on the couch.
    public var count: Int
    /// The six digits, spaced for reading out ("123 456").
    public var code: String
    /// "Code 123 456".
    public var codeLine: String
    /// What the session is doing, when that needs saying.
    public var status: String?
    /// Shown once the app stopped updating it (suspended without playing).
    public var staleNote: String
    /// The session's accent, `#rrggbb`.
    public var accent: String

    public init(
        heading: String, title: String?, detail: String?, members: [String], membersLine: String, count: Int,
        code: String, codeLine: String, status: String?, staleNote: String, accent: String
    ) {
        self.heading = heading
        self.title = title
        self.detail = detail
        self.members = members
        self.membersLine = membersLine
        self.count = count
        self.code = code
        self.codeLine = codeLine
        self.status = status
        self.staleNote = staleNote
        self.accent = accent
    }
}

#if os(iOS)
    /// A couch session on the Lock Screen and in the Dynamic Island, one activity per session.
    public struct CouchActivityAttributes: ActivityAttributes {
        public typealias ContentState = CouchActivityContent

        /// The session's code as the core has it.
        public var session: String

        public init(session: String) {
            self.session = session
        }
    }
#endif
