package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	// Verify database file was created.
	dbPath := filepath.Join(dir, "flowpbx.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Verify WAL mode is active.
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	// Verify all tables exist.
	tables := []string{
		"schema_migrations", "system_config", "extensions", "trunks",
		"inbound_numbers", "voicemail_boxes", "voicemail_messages",
		"ring_groups", "ivr_menus", "time_switches", "call_flows",
		"cdrs", "registrations", "conference_bridges",
	}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("table %s not found", table)
		}
	}

	// Verify all migrations are recorded.
	var migrationCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("counting migrations: %v", err)
	}
	if migrationCount != 19 {
		t.Errorf("migration count = %d, want 19", migrationCount)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()

	// Open twice to verify migrations don't fail on re-run.
	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open() error: %v", err)
	}
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("second Open() error: %v", err)
	}
	db2.Close()
}

func TestSystemConfigRepository(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	repo, err := NewSystemConfigRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewSystemConfigRepository() error: %v", err)
	}

	// Get non-existent key returns empty string.
	val, err := repo.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "" {
		t.Errorf("Get(nonexistent) = %q, want empty", val)
	}

	// Set and get.
	if err := repo.Set(ctx, "sip.udp_port", "5060"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	val, err = repo.Get(ctx, "sip.udp_port")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "5060" {
		t.Errorf("Get(sip.udp_port) = %q, want 5060", val)
	}

	// Update existing key.
	if err := repo.Set(ctx, "sip.udp_port", "5080"); err != nil {
		t.Fatalf("Set() update error: %v", err)
	}
	val, err = repo.Get(ctx, "sip.udp_port")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "5080" {
		t.Errorf("Get(sip.udp_port) = %q, want 5080", val)
	}

	// GetAll.
	if err := repo.Set(ctx, "http.port", "8080"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll() error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("GetAll() returned %d entries, want 2", len(all))
	}
}

func TestEncryptor(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor() error: %v", err)
	}

	plaintext := "my-secret-password-123!"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptorInvalidKeyLength(t *testing.T) {
	_, err := NewEncryptor([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}
