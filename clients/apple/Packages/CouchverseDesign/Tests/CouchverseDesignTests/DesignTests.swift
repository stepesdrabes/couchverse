import CouchverseCore
import SwiftUI
import Testing

@testable import CouchverseDesign

struct IdenticonTests {
    @Test(arguments: [
        ("admin", UInt32(1_073_681_699), 200.0),
        ("nora", 19258, 280),
        ("Štěpán", 1_352_291, 200),
        ("", 1, 40),
    ])
    func theHashMatchesTheWeb(seed: String, hash: UInt32, hue: Double) {
        #expect(Identicon.hash(seed) == hash)
        #expect(Identicon(seed: seed).hue == hue)
    }

    @Test func thePatternMatchesTheWebsSvg() {
        // the rects minidenticon("nora") draws, as (x, y)
        let web: Set<[Int]> = [
            [0, 1], [0, 3], [0, 4], [1, 0], [1, 3], [1, 4], [2, 1], [2, 4],
            [4, 1], [4, 3], [4, 4], [3, 0], [3, 3], [3, 4],
        ]
        let cells = Identicon(seed: "nora").cells
        let drawn = Set(cells.indices.filter { cells[$0] }.map { [$0 % 5, $0 / 5] })
        #expect(drawn == web)
    }

    @Test func anEmptySeedDrawsNothing() {
        #expect(Identicon(seed: "").cells.allSatisfy { !$0 })
    }
}

@MainActor
@Suite(.serialized)
struct LocalizationTests {
    @Test func stringsFollowTheDisplayLanguage() {
        defer { L10n.language = "en" }
        L10n.language = "en"
        #expect(L10n.accountsWhosWatching == "Who's watching?")
        L10n.language = "cs"
        #expect(L10n.accountsWhosWatching == "Kdo se dívá?")
    }

    @Test func unsupportedLanguagesAreIgnored() {
        defer { L10n.language = "en" }
        L10n.language = "cs"
        L10n.language = "de"
        #expect(L10n.language == "cs")
    }

    @Test(arguments: [(1, "1 sezóna"), (3, "3 sezóny"), (5, "5 sezón"), (21, "21 sezón")])
    func czechPluralsUseTheirOwnCategories(count: Int, text: String) {
        defer { L10n.language = "en" }
        L10n.language = "cs"
        #expect(L10n.catalogSeasonCount(count: count) == text)
    }

    @Test func parametersAreFilledInOrder() {
        L10n.language = "en"
        #expect(L10n.pairingRequest(device: "Apple TV", name: "Nora") == "Apple TV wants to sign in as Nora.")
        #expect(L10n.catalogSeasonCount(count: 1) == "1 season")
    }

    @Test func problemCodesHaveMessages() {
        L10n.language = "en"
        #expect(Problem(code: "invalid_credentials", detail: "").message == "Wrong username or password.")
        #expect(Problem(code: "network", detail: "").message == L10n.problemOffline)
        #expect(Problem(code: "something_new", detail: "").message == L10n.problemGeneric)
    }
}

struct AccentTests {
    @Test func hexColoursParse() throws {
        let resolved = try #require(Color(hex: "#3a6ea5")).resolve(in: EnvironmentValues())
        #expect(abs(Double(resolved.red) - 0x3A / 255.0) < 0.01)
        #expect(Color(hex: "#e5091429") != nil)
        #expect(Color(hex: "red") == nil)
        #expect(Color(hex: "#12") == nil)
    }

    @Test func aQrCodeRendersForAUrl() throws {
        let image = try #require(QRCodeView.render("http://192.168.1.5:8080/pair?code=WDJB-MJHT"))
        #expect(image.width > 20)
        #expect(image.width == image.height)
    }

    @Test func markdownInlinesKeepTheirStyles() {
        let text = Inlines.attributed([
            .text("a "), .strong([.text("b")]), .link(LinkInline(href: "https://x.y", children: [.text("c")])),
        ])
        #expect(String(text.characters) == "a bc")
        let runs = text.runs.map(\.inlinePresentationIntent)
        #expect(runs.contains(.stronglyEmphasized))
    }
}
