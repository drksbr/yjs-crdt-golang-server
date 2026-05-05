package common

import (
	"errors"
	"regexp"
	"time"
)

const (
	DefaultAddress         = ":8080"
	DefaultNamespace       = "dontpadbr3"
	DefaultSchema          = "dontpadbr3"
	DefaultDataDir         = "storage/data"
	DefaultCookieTTL       = 24 * time.Hour
	DefaultRealtimeTTL     = 5 * time.Minute
	MaxFileSizeBytes       = 50 * 1024 * 1024
	MaxAudioSizeBytes      = 25 * 1024 * 1024
	MaxDocumentUpdateBytes = 128 << 20
	MaxMultipartFormBytes  = MaxDocumentUpdateBytes + (1 << 20)
	MaxMultipartMemory     = 32 << 20
	FlushPersistTimeout    = 45 * time.Second
	MaxVersionsPerDocument = 200
	CookieNamePrefix       = "dp_auth_"
)

var (
	SchemaIdentPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	UUIDPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	VersionIDPattern   = regexp.MustCompile(`^\d{13}-[a-z0-9]+$`)
	DocUnsafePattern   = regexp.MustCompile(`[^a-z0-9_\s-]+`)

	ErrPayloadTooLarge = errors.New("payload too large")
	ErrNotFound        = errors.New("not found")

	AllowedFileMIMEs = map[string]struct{}{
		"application/msword": {},
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
		"application/vnd.ms-excel": {},
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {},
		"application/vnd.ms-powerpoint":                                             {},
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
		"application/vnd.oasis.opendocument.text":                                   {},
		"application/vnd.oasis.opendocument.spreadsheet":                            {},
		"application/vnd.oasis.opendocument.presentation":                           {},
		"application/pdf":              {},
		"text/plain":                   {},
		"text/csv":                     {},
		"text/markdown":                {},
		"image/jpeg":                   {},
		"image/png":                    {},
		"image/gif":                    {},
		"image/webp":                   {},
		"image/svg+xml":                {},
		"image/bmp":                    {},
		"image/tiff":                   {},
		"image/x-icon":                 {},
		"application/zip":              {},
		"application/x-zip-compressed": {},
		"application/x-rar-compressed": {},
		"application/x-rar":            {},
		"video/mp4":                    {},
		"video/webm":                   {},
		"video/quicktime":              {},
		"video/x-matroska":             {},
		"video/3gpp":                   {},
		"video/3gpp2":                  {},
		"video/x-msvideo":              {},
		"video/mpeg":                   {},
	}

	AllowedFileExt = map[string]struct{}{
		"doc":  {},
		"docx": {},
		"xls":  {},
		"xlsx": {},
		"ppt":  {},
		"pptx": {},
		"odt":  {},
		"ods":  {},
		"odp":  {},
		"pdf":  {},
		"txt":  {},
		"csv":  {},
		"md":   {},
		"jpg":  {},
		"jpeg": {},
		"png":  {},
		"gif":  {},
		"webp": {},
		"svg":  {},
		"bmp":  {},
		"tiff": {},
		"tif":  {},
		"ico":  {},
		"zip":  {},
		"rar":  {},
		"mp4":  {},
		"m4v":  {},
		"webm": {},
		"mov":  {},
		"mkv":  {},
		"3gp":  {},
		"3g2":  {},
		"avi":  {},
		"mpeg": {},
		"mpg":  {},
	}
)

type VisibilityMode string

const (
	VisibilityPublic     VisibilityMode = "public"
	VisibilityPublicRead VisibilityMode = "public-readonly"
	VisibilityPrivate    VisibilityMode = "private"
)

type SecuritySettingsRequest struct {
	VisibilityMode VisibilityMode `json:"visibilityMode"`
	PIN            string         `json:"pin"`
}

type VerifyPinRequest struct {
	PIN string `json:"pin"`
}

type AudioDeleteRequest struct {
	DocumentID  string `json:"documentId"`
	NoteID      string `json:"noteId"`
	Subdocument string `json:"subdocumentId"`
}

type DocumentAccessState struct {
	DocumentID     string
	IsProtected    bool
	HasPIN         bool
	VisibilityMode VisibilityMode
	HasAccess      bool
	CanEdit        bool
}

type DocumentFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
	Size         int64  `json:"size"`
	UploadedAt   int64  `json:"uploadedAt"`
	StoragePath  string `json:"-"`
}

type FilesManifest struct {
	Files []DocumentFile `json:"files"`
}

type AudioNote struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Duration    float64 `json:"duration"`
	MimeType    string  `json:"mimeType"`
	Size        int64   `json:"size"`
	CreatedAt   int64   `json:"createdAt"`
	StoragePath string  `json:"-"`
}

type DocumentVersion struct {
	ID              string  `json:"id"`
	DocumentID      string  `json:"documentId"`
	SubdocumentName *string `json:"subdocumentName,omitempty"`
	Timestamp       int64   `json:"timestamp"`
	Label           *string `json:"label,omitempty"`
	Size            int64   `json:"size"`
	CreatedBy       *string `json:"createdBy,omitempty"`
}

type DocumentChild struct {
	ID               string  `json:"id"`
	DocumentID       string  `json:"documentId"`
	ParentDocumentID string  `json:"parentDocumentId"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	OwnerSlug        *string `json:"ownerSlug,omitempty"`
	CreatedAt        int64   `json:"createdAt"`
	UpdatedAt        int64   `json:"updatedAt"`
}

type DocumentChildRequest struct {
	ID         string `json:"id"`
	DocumentID string `json:"documentId"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	OwnerSlug  string `json:"ownerSlug"`
}

type SignedToken struct {
	DocumentID string
	Scope      string
	ExpiresAt  int64
}
