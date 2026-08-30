import XCTest
@testable import DSRemote

// MARK: - Fakes

final class FakeBiometrics: BiometricAuthenticating {
    var result: Result<Void, Error> = .success(())
    private(set) var authenticateCount = 0

    func authenticate(reason: String) async throws {
        authenticateCount += 1
        try result.get()
    }
}

final class FakeKeychain: KeychainStoring {
    var stored: String?

    func saveAPIKey(_ key: String) throws { stored = key }
    func loadAPIKey() -> String? { stored }
    func deleteAPIKey() throws { stored = nil }
}

final class FakeGatewayClient: GatewayClientProtocol {
    var approveResult: Result<ControlSession, Error> = .failure(AppError.gatewayUnavailable)
    var endResult: Result<Void, Error> = .success(())
    private(set) var approveCount = 0
    private(set) var endCount = 0
    private(set) var lastEndToken: String?

    func approve(code: String, gatewayURL: URL) async throws -> ControlSession {
        approveCount += 1
        return try approveResult.get()
    }

    func endSession(sessionID: String, controlToken: String, gatewayURL: URL) async throws {
        endCount += 1
        lastEndToken = controlToken
        try endResult.get()
    }
}

// MARK: - AppViewModel tests

@MainActor
final class AppViewModelTests: XCTestCase {
    private var fakeClient: FakeGatewayClient!
    private var fakeBiometrics: FakeBiometrics!
    private var fakeKeychain: FakeKeychain!
    private var defaults: UserDefaults!
    private var viewModel: AppViewModel!

    private let validURL = "http://192.168.1.42:8080"
    private let sampleSession = ControlSession(sessionID: "sess1", state: .approved, controlToken: "tok")

    override func setUp() {
        super.setUp()
        fakeClient = FakeGatewayClient()
        fakeBiometrics = FakeBiometrics()
        fakeKeychain = FakeKeychain()
        defaults = UserDefaults(suiteName: "DSRemoteTests")!
        defaults.removePersistentDomain(forName: "DSRemoteTests")
        viewModel = AppViewModel(client: fakeClient,
                                 biometrics: fakeBiometrics,
                                 keychain: fakeKeychain,
                                 defaults: defaults)
        viewModel.gatewayURLString = validURL
        viewModel.pairingCode = "472913"
    }

    func testBiometricSuccessApprovesExactlyOnce() async {
        fakeClient.approveResult = .success(sampleSession)
        await viewModel.approve()
        XCTAssertEqual(fakeClient.approveCount, 1)
        XCTAssertEqual(viewModel.phase, .active(sampleSession))
    }

    func testBiometricFailureDoesNotCallApprove() async {
        fakeBiometrics.result = .failure(AppError.biometricFailed)
        await viewModel.approve()
        XCTAssertEqual(fakeClient.approveCount, 0)
        guard case .active = viewModel.phase else { return }
        XCTFail("no active session expected after biometric failure")
    }

    func testBiometricCancelDoesNotCallApprove() async {
        fakeBiometrics.result = .failure(AppError.biometricCancelled)
        await viewModel.approve()
        XCTAssertEqual(fakeClient.approveCount, 0)
        XCTAssertEqual(viewModel.errorMessage, AppError.biometricCancelled.errorDescription)
    }

    func testApproveErrorLeavesNoActiveSession() async {
        fakeClient.approveResult = .failure(AppError.pairingExpired)
        await viewModel.approve()
        XCTAssertEqual(fakeClient.approveCount, 1)
        guard case .active = viewModel.phase else { return }
        XCTFail("no active session expected on approve error")
    }

    func testEndSuccessClearsActiveSession() async {
        fakeClient.approveResult = .success(sampleSession)
        await viewModel.approve()
        XCTAssertEqual(viewModel.phase, .active(sampleSession))

        await viewModel.endSession()
        XCTAssertEqual(fakeClient.endCount, 1)
        XCTAssertEqual(fakeClient.lastEndToken, "tok")
        XCTAssertEqual(viewModel.phase, .pairing)
    }

    func testEndFailureRetainsActiveSession() async {
        fakeClient.approveResult = .success(sampleSession)
        await viewModel.approve()
        fakeClient.endResult = .failure(AppError.endSessionFailed)

        await viewModel.endSession()
        XCTAssertEqual(viewModel.phase, .active(sampleSession))
        XCTAssertNotNil(viewModel.errorMessage)
    }

    func testInvalidPairingCodeRejectedBeforeNetwork() async {
        viewModel.pairingCode = "12"
        fakeClient.approveResult = .success(sampleSession)
        await viewModel.approve()
        XCTAssertEqual(fakeClient.approveCount, 0)
        XCTAssertNotNil(viewModel.errorMessage)
    }

    func testGatewayURLNotSavedWhenInvalid() {
        viewModel.gatewayURLString = "not a url"
        viewModel.saveGatewayURL()
        XCTAssertEqual(viewModel.phase, .setup)
        XCTAssertNil(defaults.string(forKey: "gatewayURL"))
    }

    func testAPIKeySavedThroughKeychainAbstraction() {
        viewModel.apiKey = "sk-test-fake"
        viewModel.saveAPIKey()
        XCTAssertEqual(fakeKeychain.stored, "sk-test-fake")
    }
}
