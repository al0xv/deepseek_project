import XCTest
@testable import DSRemote

final class GenerationSettingsTests: XCTestCase {
    func testDefaultIsFlashHigh() {
        let settings = GenerationSettings.default
        XCTAssertEqual(settings.model, .flash)
        XCTAssertEqual(settings.thinking, .high)
    }

    func testModelRawValues() {
        XCTAssertEqual(DeepSeekModel.flash.rawValue, "deepseek-v4-flash")
        XCTAssertEqual(DeepSeekModel.pro.rawValue, "deepseek-v4-pro")
        XCTAssertEqual(DeepSeekModel.flash.displayName, "V4 Flash")
        XCTAssertEqual(DeepSeekModel.pro.displayName, "V4 Pro")
    }

    func testThinkingModeMapping() {
        XCTAssertFalse(ThinkingMode.off.thinkingEnabled)
        XCTAssertNil(ThinkingMode.off.reasoningEffort)
        XCTAssertTrue(ThinkingMode.low.thinkingEnabled)
        XCTAssertEqual(ThinkingMode.low.reasoningEffort, "low")
        XCTAssertEqual(ThinkingMode.high.reasoningEffort, "high")
        XCTAssertEqual(ThinkingMode.max.reasoningEffort, "max")
    }

    func testThinkingDisplayNames() {
        XCTAssertEqual(ThinkingMode.off.displayName, "Off")
        XCTAssertEqual(ThinkingMode.low.displayName, "Low")
        XCTAssertEqual(ThinkingMode.high.displayName, "High")
        XCTAssertEqual(ThinkingMode.max.displayName, "Max")
    }

    func testNoMediumOffered() {
        XCTAssertEqual(ThinkingMode.allCases.map(\.displayName), ["Off", "Low", "High", "Max"])
        XCTAssertFalse(ThinkingMode.allCases.contains { $0.displayName == "Medium" })
    }
}
