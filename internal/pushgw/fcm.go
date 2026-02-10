package pushgw

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMSender sends push notifications via Firebase Cloud Messaging.
type FCMSender struct {
	client *messaging.Client
}

// NewFCMSender initialises a Firebase app from the service-account JSON
// file at credentialsFile and returns a ready-to-use FCMSender.
// If credentialsFile is empty, the SDK falls back to
// GOOGLE_APPLICATION_CREDENTIALS or the default service account.
func NewFCMSender(ctx context.Context, credentialsFile string) (*FCMSender, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	app, err := firebase.NewApp(ctx, nil, opts...)
	if err != nil {
		return nil, fmt.Errorf("initialising firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtaining messaging client: %w", err)
	}

	slog.Info("fcm sender initialised")
	return &FCMSender{client: client}, nil
}

// Send delivers a push notification to the given FCM registration token.
// It only handles the "fcm" platform; APNs tokens are rejected.
func (f *FCMSender) Send(platform, token string, payload PushPayload) error {
	if platform != "fcm" {
		return fmt.Errorf("fcm sender: unsupported platform %q", platform)
	}

	ttl := 30 * time.Second
	msg := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"type":      payload.Type,
			"call_id":   payload.CallID,
			"caller_id": payload.CallerID,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			TTL:      &ttl,
		},
	}

	id, err := f.client.Send(context.Background(), msg)
	if err != nil {
		if messaging.IsUnregistered(err) {
			return fmt.Errorf("fcm: token no longer valid: %w", err)
		}
		return fmt.Errorf("fcm: send failed: %w", err)
	}

	slog.Debug("fcm message sent", "message_id", id, "call_id", payload.CallID)
	return nil
}
