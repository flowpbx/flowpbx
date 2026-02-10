package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SMTPConfig holds the SMTP server configuration loaded from system_config.
type SMTPConfig struct {
	Host     string // SMTP server hostname
	Port     string // SMTP port (25, 587, 465)
	From     string // From email address
	Username string // SMTP auth username
	Password string // SMTP auth password (plaintext after decryption)
	TLS      string // "none", "starttls", "tls"
}

// Valid returns true if the minimum required fields are set.
func (c SMTPConfig) Valid() bool {
	return c.Host != "" && c.Port != "" && c.From != ""
}

// VoicemailNotification describes a voicemail message for email notification.
type VoicemailNotification struct {
	To           string // recipient email address
	BoxName      string // voicemail box name
	CallerIDName string
	CallerIDNum  string
	Timestamp    time.Time
	DurationSecs int
	AudioFile    string // path to the WAV file on disk
	AttachAudio  bool   // whether to attach the WAV file
}

// Sender sends voicemail notification emails via SMTP.
type Sender struct {
	logger *slog.Logger
	// dialFunc allows injecting a custom dialer for testing.
	dialFunc func(addr string, tlsConfig *tls.Config, tlsMode string) (smtpClient, error)
}

// smtpClient abstracts the methods used from *smtp.Client for testing.
type smtpClient interface {
	Hello(localName string) error
	Extension(ext string) (bool, string)
	StartTLS(config *tls.Config) error
	Auth(a smtp.Auth) error
	Mail(from string) error
	Rcpt(to string) error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
}

// NewSender creates a new email Sender.
func NewSender(logger *slog.Logger) *Sender {
	return &Sender{
		logger:   logger.With("component", "email"),
		dialFunc: defaultDial,
	}
}

// SendVoicemailNotification sends an email notification for a new voicemail message.
func (s *Sender) SendVoicemailNotification(ctx context.Context, cfg SMTPConfig, notif VoicemailNotification) error {
	if !cfg.Valid() {
		return fmt.Errorf("smtp not configured")
	}
	if notif.To == "" {
		return fmt.Errorf("no recipient email address")
	}

	msg, err := buildMessage(cfg, notif)
	if err != nil {
		return fmt.Errorf("building email message: %w", err)
	}

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	tlsConfig := &tls.Config{ServerName: cfg.Host}

	client, err := s.dialFunc(addr, tlsConfig, cfg.TLS)
	if err != nil {
		return fmt.Errorf("connecting to smtp server: %w", err)
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}

	// STARTTLS upgrade if requested and supported.
	if strings.EqualFold(cfg.TLS, "starttls") {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	// Authenticate if credentials are provided.
	if cfg.Username != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(notif.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}

	if err := client.Quit(); err != nil {
		s.logger.Warn("smtp quit error (non-fatal)", "error", err)
	}

	s.logger.Info("voicemail notification email sent",
		"to", notif.To,
		"box", notif.BoxName,
		"caller", notif.CallerIDNum,
		"attach_audio", notif.AttachAudio,
	)

	return nil
}

// defaultDial connects to the SMTP server using either plain TCP or implicit TLS.
func defaultDial(addr string, tlsConfig *tls.Config, tlsMode string) (smtpClient, error) {
	if strings.EqualFold(tlsMode, "tls") {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, tlsConfig.ServerName)
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(addr)
	return smtp.NewClient(conn, host)
}

// buildMessage constructs the full MIME email message bytes.
func buildMessage(cfg SMTPConfig, notif VoicemailNotification) ([]byte, error) {
	var buf bytes.Buffer

	callerDisplay := notif.CallerIDNum
	if notif.CallerIDName != "" {
		callerDisplay = fmt.Sprintf("%s <%s>", notif.CallerIDName, notif.CallerIDNum)
	}

	subject := fmt.Sprintf("New voicemail from %s", callerDisplay)
	durationStr := formatDuration(notif.DurationSecs)
	body := fmt.Sprintf(
		"You have a new voicemail message in %s.\n\n"+
			"From: %s\n"+
			"Date: %s\n"+
			"Duration: %s\n",
		notif.BoxName,
		callerDisplay,
		notif.Timestamp.Format("Mon, 02 Jan 2006 3:04 PM"),
		durationStr,
	)

	if notif.AttachAudio && notif.AudioFile != "" {
		return buildMultipartMessage(cfg, notif.To, subject, body, notif.AudioFile, &buf)
	}

	// Plain text email with no attachment.
	fmt.Fprintf(&buf, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", notif.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=utf-8\r\n")
	fmt.Fprintf(&buf, "\r\n")
	buf.WriteString(body)

	return buf.Bytes(), nil
}

// buildMultipartMessage constructs a MIME multipart email with a WAV attachment.
func buildMultipartMessage(cfg SMTPConfig, to, subject, body, audioFile string, buf *bytes.Buffer) ([]byte, error) {
	writer := multipart.NewWriter(buf)

	// Write headers before the multipart body.
	fmt.Fprintf(buf, "From: %s\r\n", cfg.From)
	fmt.Fprintf(buf, "To: %s\r\n", to)
	fmt.Fprintf(buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(buf, "Content-Type: multipart/mixed; boundary=%s\r\n", writer.Boundary())
	fmt.Fprintf(buf, "\r\n")

	// Text part.
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPart, err := writer.CreatePart(textHeader)
	if err != nil {
		return nil, fmt.Errorf("creating text part: %w", err)
	}
	if _, err := textPart.Write([]byte(body)); err != nil {
		return nil, fmt.Errorf("writing text part: %w", err)
	}

	// Audio attachment.
	audioData, err := os.ReadFile(audioFile)
	if err != nil {
		return nil, fmt.Errorf("reading audio file %q: %w", audioFile, err)
	}

	filename := filepath.Base(audioFile)
	attachHeader := make(textproto.MIMEHeader)
	attachHeader.Set("Content-Type", "audio/wav; name=\""+filename+"\"")
	attachHeader.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	attachHeader.Set("Content-Transfer-Encoding", "base64")

	attachPart, err := writer.CreatePart(attachHeader)
	if err != nil {
		return nil, fmt.Errorf("creating attachment part: %w", err)
	}

	encoder := base64.NewEncoder(base64.StdEncoding, attachPart)
	if _, err := encoder.Write(audioData); err != nil {
		return nil, fmt.Errorf("encoding audio attachment: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("closing base64 encoder: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	return buf.Bytes(), nil
}

// formatDuration converts seconds into a human-readable string like "2m 15s".
func formatDuration(secs int) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	m := secs / 60
	s := secs % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
