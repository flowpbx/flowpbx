// Package prompts provides embedded default system audio prompts.
// These are G.711 u-law WAV files (8kHz, mono, 8-bit) suitable for
// direct RTP playback without transcoding.
//
// The embedded prompts are extracted to the data directory on first
// boot so they can be served by the media player. Custom prompts
// uploaded by users are stored separately in the custom/ subdirectory.
package prompts

import "embed"

// SystemFS holds the default system audio prompts embedded in the binary.
// Files are under system/ (e.g. system/default_voicemail_greeting.wav).
//
//go:embed system/*.wav
var SystemFS embed.FS

// SystemPrompts lists the filenames of all default system prompts.
// These are extracted to $DATA_DIR/prompts/system/ on first boot.
var SystemPrompts = []string{
	"default_voicemail_greeting.wav",
	"ivr_invalid_option.wav",
	"ivr_timeout.wav",
	"transfer_accept.wav",
	"followme_confirm.wav",
}
