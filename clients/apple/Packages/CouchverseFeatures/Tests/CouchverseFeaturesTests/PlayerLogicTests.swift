import CouchverseCore
import CouchverseDesign
import Foundation
import Testing

@testable import CouchverseFeatures

struct WebVTTTests {
    static let file = """
        WEBVTT
        Kind: captions

        NOTE written by the server's converter

        1
        00:00:01.000 --> 00:00:03.500 align:start position:10%
        <i>Where were you</i> &amp; why?

        00:02.000 --> 00:04.000
        - Out.
        - Again?

        3
        00:01:05.250 --> 00:01:07.000
        Last &lt;one&gt;
        """

    @Test func cuesKeepTheirTimesAndLoseTheirTags() {
        let cues = WebVTT.parse(Self.file)
        #expect(
            cues == [
                WebVTTCue(start: 1, end: 3.5, text: "Where were you & why?"),
                WebVTTCue(start: 2, end: 4, text: "- Out.\n- Again?"),
                WebVTTCue(start: 65.25, end: 67, text: "Last <one>"),
            ])
    }

    @Test(
        arguments: [
            (0.5, nil),
            (1.0, "Where were you & why?"),
            (2.5, "Where were you & why?\n- Out.\n- Again?"),
            (3.5, "- Out.\n- Again?"),
            (10.0, nil),
            (66.0, "Last <one>"),
            (67.0, nil),
        ] as [(Double, String?)])
    func theLinesShownAtATime(time: Double, text: String?) {
        #expect(WebVTT.text(at: time, in: WebVTT.parse(Self.file)) == text)
    }

    @Test func windowsLineEndingsAndCommasParse() {
        let cues = WebVTT.parse("WEBVTT\r\n\r\n00:00:01,500 --> 00:00:02,000\r\nHi\r\n")
        #expect(cues == [WebVTTCue(start: 1.5, end: 2, text: "Hi")])
    }

    @Test(arguments: ["", "12", "a:b:c", "1:2:3:4", "00:xx"])
    func malformedTimestampsAreRejected(raw: String) {
        #expect(WebVTT.timestamp(raw) == nil)
    }
}

@MainActor
struct PlayerLanguageTests {
    @Test(arguments: [
        ("cs", "cs"), ("ces", "cs"), ("cze", "cs"), ("eng", "en"), ("en-GB", "en"), ("EN", "en"), ("und", "und"),
    ])
    func streamLanguagesMatchTheCoresCodes(tag: String, expected: String) {
        #expect(PlayerController.normalized(tag) == expected)
    }
}

@MainActor
struct PlayerLabelTests {
    init() { L10n.language = "en" }

    @Test func qualityNamesTheRenditionByItsHeight() {
        #expect(PlayerLabels.quality(QualityOption(key: "720p", kind: .rendition, height: 720)) == "720p")
        #expect(PlayerLabels.quality(QualityOption(key: "auto", kind: .auto)) == "Auto")
        #expect(PlayerLabels.quality(QualityOption(key: "original", kind: .original)) == "Original")
    }

    @Test func theNextEpisodeIsNamedBySeasonAndNumber() {
        let next = NextUp(
            target: PlayTarget(kind: .episode, id: "e3"), season: 1, episode: 3, name: "Remote Control",
            countdownSeconds: 12, shuffled: false)
        #expect(PlayerLabels.nextUp(next) == "S1 E3 \u{00B7} Remote Control")
    }
}
