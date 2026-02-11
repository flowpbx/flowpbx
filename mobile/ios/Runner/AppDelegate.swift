import UIKit
import Flutter
import AVFoundation

@main
@objc class AppDelegate: FlutterAppDelegate {
    override func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        let controller = window?.rootViewController as! FlutterViewController
        let channel = FlutterMethodChannel(
            name: "com.flowpbx.mobile/audio_session",
            binaryMessenger: controller.binaryMessenger
        )

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
            default:
                result(FlutterMethodNotImplemented)
            }
        }

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
}
