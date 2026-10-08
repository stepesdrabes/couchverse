import XCTest

/// Flows against a running server, skipped unless one is given: `CV_LIVE_SERVER` (the address
/// to add), `CV_PAIRING_CODE_FILE` (where the TV writes its pairing code for the phone's test)
/// and optionally `CV_SCREENSHOTS`. xcodebuild passes them as `TEST_RUNNER_<name>`. They keep the
/// app's real stores and run in order: pair, add a second profile, switch profiles.
@MainActor
private var remote: XCUIRemote { XCUIRemote.shared }

final class LiveFlowTests: XCTestCase {
    private let environment = ProcessInfo.processInfo.environment

    @MainActor
    func test1AddAServerAndPairFromThePhone() throws {
        let app = try launch()
        let server = try XCTUnwrap(environment["CV_LIVE_SERVER"])
        let codeFile = try XCTUnwrap(environment["CV_PAIRING_CODE_FILE"])
        XCTAssert(app.staticTexts["Welcome to Couchverse"].waitForExistence(timeout: 15))
        screenshot("tv-01-welcome")
        focus(app.buttons["Add a server"])
        remote.press(.select)

        let field = app.textFields["Server address"]
        XCTAssert(field.waitForExistence(timeout: 5))
        type(server, into: field)
        screenshot("tv-02-add-server")
        focus(app.buttons["Connect"])
        remote.press(.select)

        let code = app.staticTexts["pairing-code"]
        XCTAssert(code.waitForExistence(timeout: 30))
        screenshot("tv-03-pairing")
        try code.label.write(toFile: codeFile, atomically: true, encoding: .utf8)

        XCTAssert(app.buttons["account-header"].waitForExistence(timeout: 600), "the phone approves the code")
        Thread.sleep(forTimeInterval: 2)
        screenshot("tv-04-home")
    }

    @MainActor
    func test2AddASecondProfileWithAPassword() throws {
        let app = try launch()
        // a TV always opens on "Who's watching?"
        XCTAssert(app.buttons["add-profile"].waitForExistence(timeout: 15))
        Thread.sleep(forTimeInterval: 1.5)
        screenshot("tv-05-whos-watching-one")
        focus(app.buttons["add-profile"])
        remote.press(.select)

        let username = app.textFields["Username"]
        XCTAssert(username.waitForExistence(timeout: 15))
        type(environment["CV_SECOND_USERNAME"] ?? "nora", into: username)
        type(environment["CV_SECOND_PASSWORD"] ?? "couchverse", into: app.secureTextFields["Password"])
        focus(app.buttons["Sign in"])
        remote.press(.select)
        XCTAssert(app.buttons["account-header"].waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 2)
        screenshot("tv-06-home-second")
    }

    /// Who's watching with two profiles: focus walks across them (the backdrop follows), then one
    /// is chosen and flies into the sidebar. A screen recording started outside captures it.
    @MainActor
    func test3SwitchProfiles() throws {
        let app = try launch()
        let first = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'profile-'")).firstMatch
        XCTAssert(first.waitForExistence(timeout: 15))
        Thread.sleep(forTimeInterval: 2)
        screenshot("tv-07-whos-watching")
        remote.press(.right)
        Thread.sleep(forTimeInterval: 1.5)
        screenshot("tv-08-whos-watching-focus")
        remote.press(.left)
        Thread.sleep(forTimeInterval: 1.5)
        remote.press(.select)
        // stills of the choreography; the screen recording has every frame
        for frame in 1...3 {
            screenshot("tv-09-choosing-\(frame)")
        }
        XCTAssert(app.buttons["account-header"].waitForExistence(timeout: 15))
        Thread.sleep(forTimeInterval: 2)
        screenshot("tv-10-home-after-switch")
    }

    /// From Who's watching to a title (`CV_TITLE`, Glass Harbor by default) and its playback:
    /// Movies in the sidebar, the poster, Play, the transport bar, then Back to the title.
    @MainActor
    func test4BrowseToATitleAndPlayIt() throws {
        let app = try launch()
        let first = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'profile-'")).firstMatch
        if first.waitForExistence(timeout: 10) {
            remote.press(.select)
        }
        XCTAssert(app.buttons["More info"].firstMatch.waitForExistence(timeout: 20), "the featured hero")
        Thread.sleep(forTimeInterval: 2)
        screenshot("tv-11-home")

        // the sidebar opens on Home, to the left of the content; Movies is next
        remote.press(.left)
        Thread.sleep(forTimeInterval: 1)
        remote.press(.left)
        Thread.sleep(forTimeInterval: 1)
        remote.press(.down)
        Thread.sleep(forTimeInterval: 0.5)
        remote.press(.select)
        let card = app.buttons[environment["CV_TITLE"] ?? "Glass Harbor"].firstMatch
        focusInGrid(card)
        Thread.sleep(forTimeInterval: 1)
        screenshot("tv-12-movies")
        remote.press(.select)

        let play = app.buttons["title-play"]
        XCTAssert(play.waitForExistence(timeout: 15))
        XCTAssert(play.hasFocus, "Play has the focus on a title")
        Thread.sleep(forTimeInterval: 1)
        screenshot("tv-13-title")
        remote.press(.select)

        Thread.sleep(forTimeInterval: 10)
        screenshot("tv-14-player")
        remote.press(.select)
        Thread.sleep(forTimeInterval: 1.5)
        screenshot("tv-15-transport-bar")
        // Back hides the transport bar, then closes the player; the title keeps its place under it
        remote.press(.menu)
        Thread.sleep(forTimeInterval: 1)
        remote.press(.menu)
        let back = NSPredicate(format: "hasFocus == true AND label BEGINSWITH 'Resume from'")
        expectation(for: back, evaluatedWith: play)
        waitForExpectations(timeout: 15)
        Thread.sleep(forTimeInterval: 1)
        screenshot("tv-16-title-resume")
    }

    @MainActor
    private func launch() throws -> XCUIApplication {
        guard environment["CV_LIVE_SERVER"] != nil else {
            throw XCTSkip("no CV_LIVE_SERVER")
        }
        continueAfterFailure = false
        let app = XCUIApplication()
        app.launchArguments = ["-AppleLanguages", "(en)", "-AppleLocale", "en_US"]
        app.launch()
        return app
    }

    /// Moves the focus across a grid toward `element`, by where it is on screen.
    @MainActor
    private func focusInGrid(_ element: XCUIElement, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssert(element.waitForExistence(timeout: 15), file: file, line: line)
        let focused = XCUIApplication().descendants(matching: .any).element(
            matching: NSPredicate(format: "hasFocus == true"))
        var last = CGRect.null
        for _ in 0..<24 where !element.hasFocus {
            guard focused.exists else {
                remote.press(.down)
                continue
            }
            let from = focused.frame
            let to = element.frame
            // a row may end before the column the focus is in: then go sideways first
            let stuck = from == last
            last = from
            if !stuck && to.minY > from.maxY - 10 {
                remote.press(.down)
            } else if !stuck && to.maxY < from.minY + 10 {
                remote.press(.up)
            } else {
                remote.press(to.midX > from.midX ? .right : .left)
            }
        }
        XCTAssert(element.hasFocus, "could not focus \(element)", file: file, line: line)
    }

    /// Moves focus with the remote until `element` has it.
    @MainActor
    private func focus(_ element: XCUIElement, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssert(element.waitForExistence(timeout: 10), file: file, line: line)
        for direction in [XCUIRemote.Button.down, .right, .down, .left, .up, .right, .down] where !element.hasFocus {
            for _ in 0..<4 where !element.hasFocus {
                remote.press(direction)
            }
        }
        XCTAssert(element.hasFocus, "could not focus \(element)", file: file, line: line)
    }

    @MainActor
    private func type(_ text: String, into field: XCUIElement) {
        focus(field)
        remote.press(.select)
        XCUIApplication().typeText(text)
        remote.press(.menu)
    }

    @MainActor
    private func screenshot(_ name: String) {
        let shot = XCUIScreen.main.screenshot()
        let attachment = XCTAttachment(screenshot: shot)
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
        if let directory = environment["CV_SCREENSHOTS"] {
            try? shot.pngRepresentation.write(to: URL(filePath: directory).appending(path: "\(name).png"))
        }
    }
}
