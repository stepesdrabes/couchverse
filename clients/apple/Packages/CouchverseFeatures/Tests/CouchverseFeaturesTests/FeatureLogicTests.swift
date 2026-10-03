import CouchverseCore
import Foundation
import Testing

@testable import CouchverseFeatures

struct DeepLinkTests {
    @Test(arguments: [
        ("couchverse://connect?server=https%3A%2F%2Fmedia.example.com&code=K7PQ", DeepLink.connect),
        ("couchverse://pair?code=WDJB-MJHT", .approve),
        ("COUCHVERSE://Pair?code=WDJB-MJHT", .approve),
        ("http://192.168.1.5:8080/pair?code=WDJB-MJHT", .approve),
        ("https://media.example.com/pair/?code=WDJB-MJHT", .approve),
    ])
    func linksOpenTheirScreen(url: String, link: DeepLink) {
        #expect(DeepLink(url) == link)
    }

    @Test(arguments: [
        "couchverse://title/glass-harbor",
        "https://media.example.com/pair",
        "https://media.example.com/connect?server=x&code=y",
        "https://media.example.com/watch?code=WDJB-MJHT",
        "ftp://media.example.com/pair?code=WDJB-MJHT",
        "not a url",
    ])
    func otherLinksOpenNothing(url: String) {
        #expect(DeepLink(url) == nil)
    }
}

struct UserCodeInputTests {
    @Test(arguments: [
        ("wdjb", "WDJB"),
        ("wdjbm", "WDJB-M"),
        ("wdjbmjht", "WDJB-MJHT"),
        ("WDJB-MJHT", "WDJB-MJHT"),
        (" wd jb-mj ht 42 ", "WDJB-MJHT"),
        ("WDJBMJHTXX", "WDJB-MJHT"),
        ("ččč", ""),
    ])
    func typedCodesAreGrouped(typed: String, formatted: String) {
        #expect(UserCodeInput.format(typed) == formatted)
    }

    @Test func onlyAFullCodeCanBeSubmitted() {
        #expect(UserCodeInput.isComplete("wdjbmjht"))
        #expect(!UserCodeInput.isComplete("WDJB-MJH"))
    }
}

struct PairingCountdownTests {
    @Test(arguments: [
        (UInt64(600_000), UInt64(0), "10:00"),
        (65_500, 0, "1:05"),
        (1_000_000, 999_400, "0:00"),
        (5_000, 9_000, "0:00"),
    ])
    func theCountdownReadsMinutesAndSeconds(deadline: UInt64, now: UInt64, text: String) {
        #expect(PairingPanel.countdown(deadline, now: now) == text)
    }

    @Test func theAddressIsShownWithoutItsScheme() {
        #expect(PairingPanel.displayed("http://192.168.1.5:8080/pair?code=X") == "192.168.1.5:8080/pair?code=X")
    }
}

@MainActor
struct ProfileChoreographyTests {
    @Test func theChosenAvatarFliesToTheHeaderAndLands() async {
        let choreography = ProfileChoreography()
        let card = Fixtures.account("nora", "Nora")
        let run = Task {
            await choreography.run(card: card, accounts: [card], from: CGRect(x: 100, y: 400, width: 240, height: 240))
        }
        await Task.yield()
        #expect(choreography.isRunning)
        #expect(choreography.isFlying(card.id))
        #expect(!choreography.isFlying("someone-else"))

        choreography.target = CGRect(x: 40, y: 40, width: 72, height: 72)
        // the main actor is shared with the snapshot suites, so wait for the stage, not a time
        for _ in 0..<100 where !choreography.flown {
            try? await Task.sleep(for: .milliseconds(20))
        }
        #expect(choreography.dissolved)
        #expect(choreography.flown)
        #expect(!choreography.landed)

        await run.value
        #expect(!choreography.isRunning)
        #expect(!choreography.isFlying(card.id))
        #expect(choreography.card == nil)
    }
}
