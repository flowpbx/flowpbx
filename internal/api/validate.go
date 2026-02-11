package api

import (
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// maxNameLen is the maximum length for name fields (extension names, trunk names, etc.).
const maxNameLen = 200

// maxShortStringLen is the maximum length for short identifiers (extensions, mailbox numbers).
const maxShortStringLen = 40

// maxEmailLen is the maximum length for email addresses (RFC 5321).
const maxEmailLen = 254

// maxPasswordLen is the maximum length for passwords/PINs/secrets.
const maxPasswordLen = 256

// maxURLLen is the maximum length for URL fields.
const maxURLLen = 2048

// maxHostLen is the maximum length for hostnames/IP addresses.
const maxHostLen = 253

// maxLongStringLen is the maximum length for longer text fields (TTS, file paths).
const maxLongStringLen = 1000

// maxFlowDataLen is the maximum length for call flow JSON data (512 KB).
const maxFlowDataLen = 512 * 1024

// emailRe is a basic email format regex. Not exhaustive; validates structure only.
var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// extensionRe validates extension numbers: digits only, 1-20 chars.
var extensionRe = regexp.MustCompile(`^\d{1,20}$`)

// pinRe validates PINs: digits only, 4-20 chars.
var pinRe = regexp.MustCompile(`^\d{4,20}$`)

// validateStringLen checks that a string does not exceed maxLen bytes.
// Returns an error message if invalid, empty string if OK.
func validateStringLen(field, value string, maxLen int) string {
	if utf8.RuneCountInString(value) > maxLen {
		return field + " exceeds maximum length"
	}
	return ""
}

// validateRequiredStringLen checks that a non-empty string does not exceed maxLen bytes.
func validateRequiredStringLen(field, value string, maxLen int) string {
	if value == "" {
		return field + " is required"
	}
	return validateStringLen(field, value, maxLen)
}

// validateEmail checks that a string is a valid-looking email address.
func validateEmail(field, value string) string {
	if value == "" {
		return ""
	}
	if len(value) > maxEmailLen {
		return field + " exceeds maximum length"
	}
	if !emailRe.MatchString(value) {
		return field + " is not a valid email address"
	}
	return ""
}

// validateExtensionNumber checks that an extension number is digits only.
func validateExtensionNumber(field, value string) string {
	if value == "" {
		return field + " is required"
	}
	if !extensionRe.MatchString(value) {
		return field + " must contain only digits (max 20)"
	}
	return ""
}

// validatePIN checks a PIN is digits-only and between 4-20 chars.
// Empty PINs are allowed (optional field).
func validatePIN(field, value string) string {
	if value == "" {
		return ""
	}
	if !pinRe.MatchString(value) {
		return field + " must be 4-20 digits"
	}
	return ""
}

// validateIP checks that a string is a valid IPv4 or IPv6 address.
func validateIP(field, value string) string {
	if value == "" {
		return ""
	}
	if net.ParseIP(value) == nil {
		return field + " is not a valid IP address"
	}
	return ""
}

// validateHost checks that a string looks like a valid hostname or IP.
func validateHost(field, value string) string {
	if value == "" {
		return ""
	}
	if len(value) > maxHostLen {
		return field + " exceeds maximum length"
	}
	// Accept IP addresses.
	if net.ParseIP(value) != nil {
		return ""
	}
	// Basic hostname validation: no spaces, reasonable characters.
	if strings.ContainsAny(value, " \t\n\r") {
		return field + " contains invalid characters"
	}
	return ""
}

// validateTimezone checks that a timezone string is valid per IANA.
func validateTimezone(field, value string) string {
	if value == "" {
		return ""
	}
	if len(value) > maxNameLen {
		return field + " exceeds maximum length"
	}
	_, err := time.LoadLocation(value)
	if err != nil {
		return field + " is not a valid IANA timezone"
	}
	return ""
}

// validateIntRange checks that an optional int pointer is within [min, max].
func validateIntRange(field string, value *int, min, max int) string {
	if value == nil {
		return ""
	}
	if *value < min || *value > max {
		return field + " must be between " + intToStr(min) + " and " + intToStr(max)
	}
	return ""
}

// intToStr converts an int to a string without importing strconv in a tight loop.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToStr(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// validateIPList checks that each entry in a list is a valid IP or CIDR.
func validateIPList(field string, ips []string) string {
	for i, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		// Allow CIDR notation.
		if strings.Contains(ip, "/") {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				return field + "[" + intToStr(i) + "] is not a valid IP or CIDR"
			}
			continue
		}
		if net.ParseIP(ip) == nil {
			return field + "[" + intToStr(i) + "] is not a valid IP address"
		}
	}
	return ""
}

// containsControlChars checks whether a string has control characters
// (except common whitespace like \n, \r, \t).
func containsControlChars(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

// validateNoControlChars rejects strings with control characters.
func validateNoControlChars(field, value string) string {
	if containsControlChars(value) {
		return field + " contains invalid characters"
	}
	return ""
}
