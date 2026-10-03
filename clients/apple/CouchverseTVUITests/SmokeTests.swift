import XCTest

/// A fresh install on an Apple TV simulator, driven with the remote: the welcome screen leads to
/// the add-server form and Back returns to it.
final class SmokeTests: XCTestCase {
    @MainActor
    func testTheRemoteReachesTheAddServerForm() {
        continueAfterFailure = false
        let app = XCUIApplication()
        app.launchArguments = ["-uiTesting", "-AppleLanguages", "(en)", "-AppleLocale", "en_US"]
        app.launch()
        let remote = XCUIRemote.shared

        XCTAssert(app.staticTexts["Welcome to Couchverse"].waitForExistence(timeout: 15))
        // the simulator's focus engine wakes with the first press
        remote.press(.down)
        XCTAssert(app.buttons["Add a server"].hasFocus)

        remote.press(.select)
        XCTAssert(app.textFields["Server address"].waitForExistence(timeout: 5))

        remote.press(.menu)
        XCTAssert(app.staticTexts["Welcome to Couchverse"].waitForExistence(timeout: 5))
    }
}
