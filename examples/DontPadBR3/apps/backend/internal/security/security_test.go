package security

import (
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func TestNormalizeVisibilityMode(t *testing.T) {
	t.Parallel()

	legacyHash := "hashed-pin"

	tests := []struct {
		name string
		mode string
		hash *string
		want common.VisibilityMode
	}{
		{name: "public without pin stays public", mode: "public", hash: nil, want: common.VisibilityPublic},
		{name: "private stays private", mode: "private", hash: &legacyHash, want: common.VisibilityPrivate},
		{name: "public readonly stays public readonly", mode: "public-readonly", hash: &legacyHash, want: common.VisibilityPublicRead},
		{name: "legacy public with pin becomes private", mode: "public", hash: &legacyHash, want: common.VisibilityPrivate},
		{name: "invalid mode with pin becomes private", mode: "legacy", hash: &legacyHash, want: common.VisibilityPrivate},
		{name: "invalid mode without pin becomes public", mode: "legacy", hash: nil, want: common.VisibilityPublic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NormalizeVisibilityMode(tt.mode, tt.hash)
			if got != tt.want {
				t.Fatalf("NormalizeVisibilityMode(%q, %v) = %q, want %q", tt.mode, tt.hash != nil, got, tt.want)
			}
		})
	}
}

func TestBuildDocumentAccessState(t *testing.T) {
	t.Parallel()

	hash := "hashed-pin"
	tests := []struct {
		name     string
		row      securityRow
		hasJWT   bool
		wantPIN  bool
		wantRead bool
		wantEdit bool
		wantPriv bool
	}{
		{
			name:     "public without pin allows read and edit",
			row:      securityRow{VisibilityMode: common.VisibilityPublic},
			wantRead: true,
			wantEdit: true,
		},
		{
			name:     "public readonly with pin allows read but blocks edit",
			row:      securityRow{VisibilityMode: common.VisibilityPublicRead, PinHash: &hash},
			wantPIN:  true,
			wantRead: true,
		},
		{
			name:     "private with pin blocks read and edit without cookie",
			row:      securityRow{VisibilityMode: common.VisibilityPrivate, PinHash: &hash},
			wantPIN:  true,
			wantPriv: true,
		},
		{
			name:     "private with cookie allows read and edit",
			row:      securityRow{VisibilityMode: common.VisibilityPrivate, PinHash: &hash},
			hasJWT:   true,
			wantPIN:  true,
			wantRead: true,
			wantEdit: true,
			wantPriv: true,
		},
		{
			name:     "public readonly with cookie allows edit",
			row:      securityRow{VisibilityMode: common.VisibilityPublicRead, PinHash: &hash},
			hasJWT:   true,
			wantPIN:  true,
			wantRead: true,
			wantEdit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildDocumentAccessState("doc", tt.row, tt.hasJWT)
			if got.HasPIN != tt.wantPIN {
				t.Fatalf("HasPIN = %v, want %v", got.HasPIN, tt.wantPIN)
			}
			if got.HasAccess != tt.wantRead {
				t.Fatalf("HasAccess = %v, want %v", got.HasAccess, tt.wantRead)
			}
			if got.CanEdit != tt.wantEdit {
				t.Fatalf("CanEdit = %v, want %v", got.CanEdit, tt.wantEdit)
			}
			if got.IsProtected != tt.wantPriv {
				t.Fatalf("IsProtected = %v, want %v", got.IsProtected, tt.wantPriv)
			}
		})
	}
}
