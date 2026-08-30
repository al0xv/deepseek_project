import XCTest
@testable import DSRemote

final class QRPairingParserTests: XCTestCase {
    private let valid = "dsremote://pair?v=1&code=472913"

    func testValidPayload() throws {
        let payload = try QRPairingParser.parse(valid)
        XCTAssertEqual(payload.pairingCode, "472913")
        XCTAssertEqual(payload.version, 1)
    }

    func testMalformedRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("not a url"))
        XCTAssertThrowsError(try QRPairingParser.parse(""))
    }

    func testWrongSchemeRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("https://pair?v=1&code=472913"))
    }

    func testWrongHostRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://other?code=472913"))
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair.other?v=1&code=472913"))
    }

    func testUnsupportedVersionRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?v=2&code=472913"))
    }

    func testMissingVersionRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?code=472913"))
    }

    func testMissingCodeRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?v=1"))
    }

    func testFiveDigitsRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?v=1&code=47291"))
    }

    func testSevenDigitsRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?v=1&code=4729134"))
    }

    func testLettersRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?v=1&code=abcdef"))
    }

    func testIrrelevantQRRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("https://google.com"))
        XCTAssertThrowsError(try QRPairingParser.parse("hello"))
    }

    func testExtraQueryParamRejected() {
        XCTAssertThrowsError(try QRPairingParser.parse("dsremote://pair?v=1&code=472913&foo=bar"))
    }

    func testInvalidThrowsNotDSRemoteQR() {
        XCTAssertThrowsError(try QRPairingParser.parse("https://google.com")) { error in
            XCTAssertEqual(error as? AppError, AppError.notDSRemoteQR)
        }
    }
}
