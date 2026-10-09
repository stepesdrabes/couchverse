import XCTest

/// Flows against a running server, skipped unless one is given: `CV_LIVE_SERVER` (the address
/// to add) and optionally `CV_SCREENSHOTS` (a directory for screenshots). xcodebuild passes them
/// to the test runner as `TEST_RUNNER_CV_LIVE_SERVER` and `TEST_RUNNER_CV_SCREENSHOTS`. They keep
/// the app's real stores, so they run in order on one simulator: sign in, then approve the TV.
final class LiveFlowTests: XCTestCase {
    private let environment = ProcessInfo.processInfo.environment

    @MainActor
    func test1AddAServerAndSignInWithAPassword() throws {
        let app = try launch()
        let server = try XCTUnwrap(environment["CV_LIVE_SERVER"])
        XCTAssert(app.staticTexts["Welcome to Couchverse"].waitForExistence(timeout: 15))
        screenshot("phone-01-welcome")
        app.buttons["Add a server"].tap()

        let field = app.textFields["Server address"]
        XCTAssert(field.waitForExistence(timeout: 5))
        field.tap()
        field.typeText(server)
        screenshot("phone-02-add-server")
        app.buttons["Connect"].tap()

        let username = app.textFields["Username"]
        XCTAssert(username.waitForExistence(timeout: 20))
        screenshot("phone-03-sign-in")
        username.tap()
        username.typeText(environment["CV_USERNAME"] ?? "admin")
        let password = app.secureTextFields["Password"]
        password.tap()
        password.typeText((environment["CV_PASSWORD"] ?? "admin") + "\n")

        XCTAssert(app.buttons["account-switcher"].waitForExistence(timeout: 20))
        // the system offers to save the password; not this time
        let notNow = app.buttons["Not Now"]
        if notNow.waitForExistence(timeout: 5) {
            notNow.tap()
        }
        XCTAssert(app.buttons["More info"].firstMatch.waitForExistence(timeout: 20), "the featured hero")
        Thread.sleep(forTimeInterval: 2)
        screenshot("phone-04-home")
    }

    /// Approves the code the TV shows: typed in Settings, as someone holding the remote would read
    /// it out. The TV's test writes the code to `CV_PAIRING_CODE_FILE`.
    @MainActor
    func test2ApproveTheTVsPairingCode() throws {
        let app = try launch()
        let file = try XCTUnwrap(environment["CV_PAIRING_CODE_FILE"], "set CV_PAIRING_CODE_FILE")
        XCTAssert(app.buttons["account-switcher"].waitForExistence(timeout: 20))
        app.tabBars.buttons["Settings"].firstMatch.tap()
        // below the profile, accounts and servers: a lazy list makes the row only once it scrolls in
        let approve = app.buttons["approve-device"]
        for _ in 0..<6 where !approve.isHittable {
            app.swipeUp()
        }
        XCTAssert(approve.waitForExistence(timeout: 10))
        screenshot("phone-05-settings")
        approve.tap()

        var code: String?
        let deadline = Date.now.addingTimeInterval(600)
        while code == nil, Date.now < deadline {
            code = (try? String(contentsOfFile: file, encoding: .utf8))?
                .trimmingCharacters(in: .whitespacesAndNewlines)
            if code?.isEmpty ?? true {
                code = nil
                Thread.sleep(forTimeInterval: 1)
            }
        }
        let field = app.textFields["Code shown on the device"]
        XCTAssert(field.waitForExistence(timeout: 10))
        field.tap()
        field.typeText(try XCTUnwrap(code))
        screenshot("phone-06-approve-code")
        app.buttons["Continue"].tap()

        let approveButton = app.buttons["approve-pairing"]
        XCTAssert(approveButton.waitForExistence(timeout: 15))
        screenshot("phone-07-approve-request")
        approveButton.tap()
        XCTAssert(
            app.staticTexts.containing(NSPredicate(format: "label BEGINSWITH 'Approved'")).firstMatch.waitForExistence(
                timeout: 15))
        screenshot("phone-08-approved")
    }

    /// Browse to a title (`CV_TITLE`, Glass Harbor by default), play it, open the player's
    /// options, close the player and find the title resuming where it stopped.
    @MainActor
    func test3BrowseToATitleAndPlayIt() throws {
        let app = try launch()
        XCTAssert(app.buttons["account-switcher"].waitForExistence(timeout: 20))
        app.tabBars.buttons["Browse"].firstMatch.tap()
        let card = app.buttons[environment["CV_TITLE"] ?? "Glass Harbor"].firstMatch
        XCTAssert(card.waitForExistence(timeout: 15))
        Thread.sleep(forTimeInterval: 1)
        screenshot("phone-09-browse")
        card.tap()

        let play = app.buttons["title-play"]
        XCTAssert(play.waitForExistence(timeout: 15))
        Thread.sleep(forTimeInterval: 1)
        screenshot("phone-10-title")
        play.tap()

        let options = app.buttons["player-options"]
        XCTAssert(options.waitForExistence(timeout: 20), "the player shows its options")
        Thread.sleep(forTimeInterval: 8)
        app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.35)).tap()
        screenshot("phone-11-player")
        options.tap()
        // the couch is always on offer; Quality only when the title has a ladder besides its original
        XCTAssert(app.buttons["Start a couch session"].firstMatch.waitForExistence(timeout: 5))
        screenshot("phone-12-player-options")
        app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.9)).tap()

        // the system player's own close
        if !app.buttons["Close"].firstMatch.isHittable {
            app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.35)).tap()
        }
        app.buttons["Close"].firstMatch.tap()
        // the title stays under the player; back in front, it offers to resume
        let resume = NSPredicate(format: "hittable == true AND label BEGINSWITH 'Resume from'")
        expectation(for: resume, evaluatedWith: play)
        waitForExpectations(timeout: 15)
        screenshot("phone-13-title-resume")
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
