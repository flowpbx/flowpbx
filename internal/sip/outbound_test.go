package sip

import (
	"testing"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

func TestApplyPrefixRules(t *testing.T) {
	tests := []struct {
		name   string
		number string
		strip  int
		add    string
		want   string
	}{
		{
			name:   "no transformation",
			number: "61412345678",
			strip:  0,
			add:    "",
			want:   "61412345678",
		},
		{
			name:   "strip leading digit",
			number: "0412345678",
			strip:  1,
			add:    "",
			want:   "412345678",
		},
		{
			name:   "add prefix only",
			number: "412345678",
			strip:  0,
			add:    "61",
			want:   "61412345678",
		},
		{
			name:   "strip and add prefix",
			number: "07700900000",
			strip:  1,
			add:    "0044",
			want:   "00447700900000",
		},
		{
			name:   "strip multiple digits",
			number: "001234567890",
			strip:  3,
			add:    "+",
			want:   "+234567890",
		},
		{
			name:   "strip all digits results in empty then add prefix",
			number: "0",
			strip:  1,
			add:    "1234",
			want:   "1234",
		},
		{
			name:   "strip more than length results in empty",
			number: "12",
			strip:  5,
			add:    "",
			want:   "",
		},
		{
			name:   "strip more than length then add prefix",
			number: "12",
			strip:  5,
			add:    "999",
			want:   "999",
		},
		{
			name:   "empty number with no rules",
			number: "",
			strip:  0,
			add:    "",
			want:   "",
		},
		{
			name:   "empty number with add prefix",
			number: "",
			strip:  0,
			add:    "61",
			want:   "61",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPrefixRules(tt.number, tt.strip, tt.add)
			if got != tt.want {
				t.Errorf("applyPrefixRules(%q, %d, %q) = %q, want %q",
					tt.number, tt.strip, tt.add, got, tt.want)
			}
		})
	}
}

func TestBuildOutboundCallerID(t *testing.T) {
	tests := []struct {
		name       string
		ext        *models.Extension
		trunk      *models.Trunk
		wantName   string
		wantNum    string
		wantSource callerIDSource
	}{
		{
			name: "extension only — no trunk CID set",
			ext: &models.Extension{
				Name:      "Alice",
				Extension: "101",
			},
			trunk:      &models.Trunk{},
			wantName:   "Alice",
			wantNum:    "101",
			wantSource: callerIDFromExtension,
		},
		{
			name: "trunk overrides both name and number",
			ext: &models.Extension{
				Name:      "Alice",
				Extension: "101",
			},
			trunk: &models.Trunk{
				CallerIDName: "Main Office",
				CallerIDNum:  "+61299998888",
			},
			wantName:   "Main Office",
			wantNum:    "+61299998888",
			wantSource: callerIDFromTrunk,
		},
		{
			name: "trunk overrides number only",
			ext: &models.Extension{
				Name:      "Alice",
				Extension: "101",
			},
			trunk: &models.Trunk{
				CallerIDNum: "+61299998888",
			},
			wantName:   "Alice",
			wantNum:    "+61299998888",
			wantSource: callerIDFromTrunk,
		},
		{
			name: "trunk overrides name only",
			ext: &models.Extension{
				Name:      "Alice",
				Extension: "101",
			},
			trunk: &models.Trunk{
				CallerIDName: "Main Office",
			},
			wantName:   "Main Office",
			wantNum:    "101",
			wantSource: callerIDFromTrunk,
		},
		{
			name:       "nil extension — trunk CID only",
			ext:        nil,
			trunk:      &models.Trunk{CallerIDName: "Trunk", CallerIDNum: "+61200000000"},
			wantName:   "Trunk",
			wantNum:    "+61200000000",
			wantSource: callerIDFromTrunk,
		},
		{
			name:       "nil extension and no trunk CID — empty values",
			ext:        nil,
			trunk:      &models.Trunk{},
			wantName:   "",
			wantNum:    "",
			wantSource: callerIDFromExtension,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotNum, gotSource := buildOutboundCallerID(tt.ext, tt.trunk)
			if gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}
			if gotNum != tt.wantNum {
				t.Errorf("number = %q, want %q", gotNum, tt.wantNum)
			}
			if gotSource != tt.wantSource {
				t.Errorf("source = %q, want %q", gotSource, tt.wantSource)
			}
		})
	}
}
