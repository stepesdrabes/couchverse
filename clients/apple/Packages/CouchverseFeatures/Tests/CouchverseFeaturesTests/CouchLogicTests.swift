import CouchverseCore
import CouchverseDesign
import Foundation
import Testing
import UIKit

@testable import CouchverseFeatures

struct CouchLinkTests {
    @Test(arguments: [
        ("couchverse://couch/123456", "123456"),
        ("COUCHVERSE://Couch/654321/", "654321"),
        ("https://media.example.com/couch/123456", "123456"),
        ("http://10.0.2.2:8092/couch/654321/?from=qr", "654321"),
    ])
    func couchLinksCarryTheirCode(url: String, code: String) {
        #expect(CouchLink.code(url) == code)
        #expect(DeepLink(url) == .couch)
    }

    @Test(arguments: [
        "couchverse://couch/12345",
        "couchverse://couch/1234567",
        "https://media.example.com/couch/12a456",
        "https://media.example.com/watch/couch/123456",
        "https://media.example.com/pair?code=WDJB-MJHT",
        "couchverse://title/glass-harbor",
        "ftp://media.example.com/couch/123456",
    ])
    func otherLinksCarryNoCode(url: String) {
        #expect(CouchLink.code(url) == nil)
        #expect(DeepLink(url) != .couch)
    }

    @Test(
        arguments: [
            ("couchverse://couch/123456", nil),
            ("couchverse://couch/123456?server=http%3A%2F%2F192.168.1.5%3A8080", "http://192.168.1.5:8080"),
            ("couchverse://couch/123456?server=", nil),
            ("https://media.example.com/couch/123456", "https://media.example.com"),
            ("http://10.0.2.2:8092/couch/654321/?from=qr#top", "http://10.0.2.2:8092"),
            ("http://[fd00::5]:8080/couch/654321", "http://[fd00::5]:8080"),
        ] as [(String, String?)])
    func couchLinksNameTheirServer(url: String, server: String?) {
        #expect(CouchLink.invite(url)?.server == server)
    }
}

struct CouchCodeInputTests {
    @Test(arguments: [
        ("123456", "123456"),
        (" 123 456 ", "123456"),
        ("12-34-56-78", "123456"),
        ("abc", ""),
        ("١٢٣٤٥٦", ""),
    ])
    func typedCodesKeepSixDigits(typed: String, formatted: String) {
        #expect(CouchCodeInput.format(typed) == formatted)
    }

    @Test func onlyAFullCodeJoins() {
        #expect(CouchCodeInput.isComplete("123456"))
        #expect(!CouchCodeInput.isComplete("12345"))
        #expect(!CouchCodeInput.isComplete("12345a"))
    }
}

struct ReactionTests {
    @Test func recentReactionsComeFirstWithoutRepeats() {
        let choices = Reactions.choices(recent: ["🦄", "🔥"])
        #expect(Array(choices.prefix(3)) == ["🦄", "🔥", "❤️"])
        #expect(Set(choices).count == choices.count)
        #expect(choices.count == Reactions.quick.count + 1)
    }

    @Test func theKeyboardSendsEachEmojiAndNothingElse() {
        #expect(Reactions.emoji(in: "a😂 b🇨🇿1️⃣7#👍🏽❤️👩‍👩‍👧") == ["😂", "🇨🇿", "1️⃣", "👍🏽", "❤️", "👩‍👩‍👧"])
        #expect(Reactions.emoji(in: "hello 42") == [])
    }
}

@MainActor
struct CouchLabelTests {
    init() { L10n.language = "en" }

    @Test(
        arguments: [
            ("following", nil),
            ("host-paused", "Host paused"),
            ("resynced", "Resynced with the host"),
            ("waiting", "Štěpán is choosing what to watch"),
            ("away", "The host stepped away"),
            ("connecting", "Connecting to the couch..."),
            ("hosting", nil),
            ("remote", nil),
            ("ended", "The host ended the session"),
        ] as [(String, String?)])
    func theStatusLineSaysWhatTheSessionIsDoing(state: String, line: String?) {
        #expect(CouchLabels.status(Fixtures.couch(state)) == line)
    }

    @Test func aFailedJoinSaysWhy() {
        #expect(Problem(code: "no_session", detail: "").message == L10n.couchJoinFailed)
        #expect(
            Problem(code: "not_host", detail: "").message == "Only the host's account can use the couch as a remote.")
        #expect(Problem(code: "session_full", detail: "").message == L10n.problemSessionFull)
    }

    @Test func codesAreReadInTwoHalves() {
        #expect(CouchLabels.spaced("123456") == "123 456")
        #expect(CouchLabels.spaced("12345") == "12345")
    }

    @Test func voiceOverSaysACodeDigitByDigit() {
        #expect(CouchLabels.spokenDigits("123456") == "1 2 3 4 5 6")
        #expect(L10n.couchCode(code: CouchLabels.spokenDigits("042")) == "Code 0 4 2")
    }

    @Test func membersShowTheirPlaceOnTheCouch() {
        let members = Fixtures.hostingMembers
        #expect(CouchLabels.badges(members[0], hostAway: false) == "Host · You")
        #expect(CouchLabels.badges(members[0], hostAway: true) == "The host stepped away · You")
        #expect(CouchLabels.badges(members[1], hostAway: false) == "")
        #expect(CouchLabels.badges(members[2], hostAway: false) == "Paused")
        #expect(CouchPanel.seated(Array(Fixtures.followingMembers.reversed())).first?.host == true)
    }

    @Test func aRemoteRunsTheHostsClockOnWhilePlaying() {
        let playing = Fixtures.couch("remote")
        #expect(CouchLabels.position(playing, now: 2_500) == 756.5)
        #expect(CouchLabels.position(Fixtures.couch("remote-paused"), now: 2_500) == 754)
        // a report from after the clock reading moves nothing back
        let later = CouchView(
            status: .open, role: .remote, members: [], playing: true, positionSeconds: 754, positionAtMs: 9_000,
            hostAway: false, waiting: false, localPaused: false, reactions: [], recentEmojis: [], resynced: false)
        #expect(CouchLabels.position(later, now: 2_500) == 754)
    }
}

@MainActor
struct AppCoverTests {
    let idle = Fixtures.couch("idle")

    @Test func thePlayerComesFirst() {
        let requests = CoverRequests()
        requests.joining = CouchInvite(code: "123456")
        #expect(requests.cover(playing: true, couch: Fixtures.couch("remote"), ready: true) == .player)
    }

    @Test func aFollowerWaitsForTheHostAndARemoteSteers() {
        let requests = CoverRequests()
        #expect(requests.cover(playing: false, couch: Fixtures.couch("waiting"), ready: true) == .waiting)
        #expect(requests.cover(playing: false, couch: Fixtures.couch("connecting"), ready: true) == .waiting)
        #expect(requests.cover(playing: false, couch: Fixtures.couch("remote"), ready: true) == .remote)
        #expect(requests.cover(playing: false, couch: Fixtures.couch("hosting"), ready: true) == nil)
    }

    @Test func aJoinScreenWaitsForAnAccountAndCloses() {
        let requests = CoverRequests()
        let invite = CouchInvite(code: "123456")
        requests.joining = invite
        #expect(requests.cover(playing: false, couch: idle, ready: false) == nil)
        #expect(requests.cover(playing: false, couch: idle, ready: true) == .join(invite))
        requests.seated(.follower)
        #expect(requests.joining == nil)
    }

    @Test func aJoinNamingItsServerNeedsNoAccount() {
        let requests = CoverRequests()
        let link = CouchInvite(code: "123456", server: "http://192.168.1.5:8080")
        requests.joining = link
        #expect(requests.cover(playing: false, couch: idle, ready: false) == .join(link))
        // the welcome screen's: the server is typed
        requests.joining = CouchInvite(server: "")
        #expect(requests.cover(playing: false, couch: idle, ready: false) == .join(CouchInvite(server: "")))
        // a guest seated as a follower waits for the host like anyone
        #expect(requests.cover(playing: false, couch: Fixtures.couch("waiting"), ready: false) == .waiting)
    }

    @Test func theEndIsShownToAViewerUntilSeen() {
        let requests = CoverRequests()
        let ended = Fixtures.couch("ended")
        #expect(requests.cover(playing: false, couch: ended, ready: true) == nil, "never on this couch")
        requests.seated(.follower)
        #expect(requests.cover(playing: false, couch: ended, ready: true) == .ended)
        requests.endSeen = true
        #expect(requests.cover(playing: false, couch: ended, ready: true) == nil)
        requests.seated(.host)
        #expect(requests.cover(playing: false, couch: ended, ready: true) == nil, "a host keeps watching")
    }

    @Test func leavingNeedsNoWord() {
        let requests = CoverRequests()
        requests.seated(.remote)
        let left = CouchView(
            status: .ended, members: [], playing: false, positionSeconds: 0, positionAtMs: 0, hostAway: false,
            waiting: false, localPaused: false, reactions: [], recentEmojis: [], resynced: false, ended: "left")
        #expect(requests.cover(playing: false, couch: left, ready: true) == nil)
    }
}

struct LocalPauseTests {
    @Test func onlyWhatTheCoreDidNotAskForIsTheViewersOwn() {
        var pause = LocalPause()
        #expect(pause.observed(playing: false) == nil, "nothing asked yet")
        pause.commanded(playing: true)
        #expect(pause.observed(playing: true) == nil)
        #expect(pause.observed(playing: false) == true, "the viewer paused")
        #expect(pause.observed(playing: false) == nil, "said once")
        #expect(pause.observed(playing: true) == false, "the viewer resumed")
        pause.commanded(playing: false)
        #expect(pause.observed(playing: false) == nil, "the core paused it")
    }
}

@MainActor
struct CouchMenuTests {
    init() { L10n.language = "en" }

    @Test func nothingWhileCouchSessionsAreOff() {
        #expect(CouchMenu(Fixtures.couch("hosting"), enabled: false) == nil)
    }

    @Test func startingASessionThenItsCodeAndTheWayOff() {
        #expect(CouchMenu(Fixtures.couch("idle"), enabled: true)?.entries == ["Start a couch session"])
        #expect(CouchMenu(Fixtures.couch("hosting"), enabled: true)?.entries == ["Code 123 456", "End session"])
        #expect(CouchMenu(Fixtures.couch("following"), enabled: true)?.entries == ["On the couch · 3", "Leave couch"])
    }

    #if os(tvOS)
        @Test func theTransportBarOffersTheCouchAndItsReactions() throws {
            let menu = try #require(CouchMenu(Fixtures.couch("hosting"), enabled: true))
            let items = menu.items(send: { _ in }, panel: {})
            #expect(items.map(\.title) == ["Couch session", "Send a reaction"])
            let couch = try #require(items.first as? UIMenu)
            #expect(couch.children.map(\.title) == ["Code 123 456", "End session"])
            let idle = try #require(CouchMenu(Fixtures.couch("idle"), enabled: true))
            #expect(idle.items(send: { _ in }, panel: {}).map(\.title) == ["Couch session"])
        }
    #endif
}
