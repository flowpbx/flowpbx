import UIKit
import Flutter
import AVFoundation
import CallKit

@main
@objc class AppDelegate: FlutterAppDelegate {
    private var audioChannel: FlutterMethodChannel?
    private let callKitManager = CallKitManager()

    override func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        let controller = window?.rootViewController as! FlutterViewController
        let channel = FlutterMethodChannel(
            name: "com.flowpbx.mobile/audio_session",
            binaryMessenger: controller.binaryMessenger
        )
        audioChannel = channel

        channel.setMethodCallHandler { [weak self] (call, result) in
            switch call.method {
            case "configure":
                self?.configureAudioSession(result: result)
            case "activate":
                self?.activateAudioSession(result: result)
            case "deactivate":
                self?.deactivateAudioSession(result: result)
            case "setSpeaker":
                if let args = call.arguments as? [String: Any],
                   let enabled = args["enabled"] as? Bool {
                    self?.setSpeaker(enabled: enabled, result: result)
                } else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing 'enabled' argument", details: nil))
                }
            case "getAudioRoute":
                result(self?.currentAudioRoute() ?? "earpiece")
            default:
                result(FlutterMethodNotImplemented)
            }
        }

        // Proximity sensor platform channel.
        let proximityChannel = FlutterMethodChannel(
            name: "com.flowpbx.mobile/proximity",
            binaryMessenger: controller.binaryMessenger
        )

        proximityChannel.setMethodCallHandler { (call, result) in
            switch call.method {
            case "enable":
                UIDevice.current.isProximityMonitoringEnabled = true
                result(true)
            case "disable":
                UIDevice.current.isProximityMonitoringEnabled = false
                result(true)
            default:
                result(FlutterMethodNotImplemented)
            }
        }

        // CallKit platform channel.
        let callKitChannel = FlutterMethodChannel(
            name: "com.flowpbx.mobile/callkit",
            binaryMessenger: controller.binaryMessenger
        )
        callKitManager.setChannel(callKitChannel)

        callKitChannel.setMethodCallHandler { [weak self] (call, result) in
            guard let self = self else {
                result(FlutterError(code: "UNAVAILABLE", message: "AppDelegate released", details: nil))
                return
            }
            switch call.method {
            case "reportIncomingCall":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr),
                      let handle = args["handle"] as? String else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid or handle", details: nil))
                    return
                }
                let displayName = args["displayName"] as? String
                self.callKitManager.reportIncomingCall(uuid: uuid, handle: handle, displayName: displayName) { error in
                    if let error = error {
                        result(FlutterError(code: "CALLKIT_ERROR", message: error.localizedDescription, details: nil))
                    } else {
                        result(true)
                    }
                }
            case "reportOutgoingCall":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr),
                      let handle = args["handle"] as? String else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid or handle", details: nil))
                    return
                }
                self.callKitManager.reportOutgoingCall(uuid: uuid, handle: handle)
                result(true)
            case "reportOutgoingCallConnected":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr) else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid", details: nil))
                    return
                }
                self.callKitManager.reportOutgoingCallConnected(uuid: uuid)
                result(true)
            case "reportCallEnded":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr) else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid", details: nil))
                    return
                }
                let reasonInt = args["reason"] as? Int ?? 1
                let reason: CXCallEndedReason
                switch reasonInt {
                case 2: reason = .failed
                case 3: reason = .unanswered
                case 4: reason = .declinedElsewhere
                case 5: reason = .answeredElsewhere
                default: reason = .remoteEnded
                }
                self.callKitManager.reportCallEnded(uuid: uuid, reason: reason)
                result(true)
            case "endCall":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr) else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid", details: nil))
                    return
                }
                self.callKitManager.endCall(uuid: uuid)
                result(true)
            case "setMuted":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr),
                      let muted = args["muted"] as? Bool else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid or muted", details: nil))
                    return
                }
                self.callKitManager.setMuted(uuid: uuid, muted: muted)
                result(true)
            case "setHeld":
                guard let args = call.arguments as? [String: Any],
                      let uuidStr = args["uuid"] as? String,
                      let uuid = UUID(uuidString: uuidStr),
                      let held = args["held"] as? Bool else {
                    result(FlutterError(code: "INVALID_ARGS", message: "Missing uuid or held", details: nil))
                    return
                }
                self.callKitManager.setHeld(uuid: uuid, held: held)
                result(true)
            default:
                result(FlutterMethodNotImplemented)
            }
        }

        // Observe audio route changes (Bluetooth connect/disconnect, headset plug).
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(audioRouteChanged),
            name: AVAudioSession.routeChangeNotification,
            object: nil
        )

        GeneratedPluginRegistrant.register(with: self)
        return super.application(application, didFinishLaunchingWithOptions: launchOptions)
    }

    /// Configure AVAudioSession for VoIP calling.
    /// Category: playAndRecord — enables simultaneous input and output.
    /// Mode: voiceChat — optimises for two-way voice (echo cancellation, AGC).
    /// Options: allowBluetooth + allowBluetoothA2DP for headset support,
    ///          defaultToSpeaker = false so earpiece is default.
    private func configureAudioSession(result: @escaping FlutterResult) {
        let session = AVAudioSession.sharedInstance()
        do {
            try session.setCategory(
                .playAndRecord,
                mode: .voiceChat,
                options: [.allowBluetooth, .allowBluetoothA2DP]
            )
            result(true)
        } catch {
            result(FlutterError(
                code: "AUDIO_SESSION_ERROR",
                message: "Failed to configure audio session: \(error.localizedDescription)",
                details: nil
            ))
        }
    }

    /// Activate the audio session when a call starts.
    private func activateAudioSession(result: @escaping FlutterResult) {
        let session = AVAudioSession.sharedInstance()
        do {
            try session.setActive(true, options: [])
            result(true)
        } catch {
            result(FlutterError(
                code: "AUDIO_SESSION_ERROR",
                message: "Failed to activate audio session: \(error.localizedDescription)",
                details: nil
            ))
        }
    }

    /// Deactivate the audio session when a call ends.
    /// Uses .notifyOthersOnDeactivation so other apps can resume playback.
    private func deactivateAudioSession(result: @escaping FlutterResult) {
        let session = AVAudioSession.sharedInstance()
        do {
            try session.setActive(false, options: [.notifyOthersOnDeactivation])
            result(true)
        } catch {
            // Deactivation can fail if another session is active; not critical.
            result(true)
        }
    }

    /// Override the audio output port to speaker or earpiece.
    private func setSpeaker(enabled: Bool, result: @escaping FlutterResult) {
        let session = AVAudioSession.sharedInstance()
        do {
            if enabled {
                try session.overrideOutputAudioPort(.speaker)
            } else {
                try session.overrideOutputAudioPort(.none)
            }
            result(true)
        } catch {
            result(FlutterError(
                code: "AUDIO_SESSION_ERROR",
                message: "Failed to set speaker: \(error.localizedDescription)",
                details: nil
            ))
        }
    }

    /// Determine the current audio output route.
    private func currentAudioRoute() -> String {
        let session = AVAudioSession.sharedInstance()
        guard let output = session.currentRoute.outputs.first else {
            return "earpiece"
        }
        switch output.portType {
        case .builtInSpeaker:
            return "speaker"
        case .bluetoothA2DP, .bluetoothLE, .bluetoothHFP:
            return "bluetooth"
        case .headphones, .headsetMic:
            return "headset"
        default:
            return "earpiece"
        }
    }

    /// Callback when the audio route changes (e.g. Bluetooth connected, headset plugged).
    @objc private func audioRouteChanged(notification: Notification) {
        let route = currentAudioRoute()
        DispatchQueue.main.async { [weak self] in
            self?.audioChannel?.invokeMethod("onAudioRouteChanged", arguments: route)
        }
    }
}
