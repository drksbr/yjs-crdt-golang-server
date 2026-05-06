package common

import (
	"os"
	"path"
	"path/filepath"
	"time"
)

type StoragePaths struct {
	Root string
}

func (p StoragePaths) DocumentDir(documentID string) string {
	return filepath.Join(p.Root, "documents", documentID)
}

func (p StoragePaths) DocumentPrefix(documentID string) string {
	return path.Join("documents", documentID) + "/"
}

func (p StoragePaths) LegacyDocumentPrefix(documentID string) string {
	return path.Join(documentID) + "/"
}

func (p StoragePaths) UploadDir(documentID string, subdoc *string, ensure bool) string {
	base := p.DocumentDir(documentID)
	if subdoc != nil {
		base = filepath.Join(base, *subdoc)
	}
	if ensure {
		_ = os.MkdirAll(base, 0o755)
	}
	return base
}

func (p StoragePaths) AudioDir(documentID string, subdoc *string, ensure bool) string {
	base := filepath.Join(p.DocumentDir(documentID), "audio-notes")
	if subdoc != nil {
		base = filepath.Join(base, *subdoc)
	}
	if ensure {
		_ = os.MkdirAll(base, 0o755)
	}
	return base
}

func (p StoragePaths) DatedUploadDir(documentID string, subdoc *string, ts time.Time, ensure bool) string {
	return p.datedMediaDir(documentID, subdoc, "files", ts, ensure)
}

func (p StoragePaths) UploadKey(documentID string, subdoc *string, storedName string) string {
	base := path.Join("documents", documentID)
	if subdoc != nil {
		base = path.Join(base, *subdoc)
	}
	return path.Join(base, storedName)
}

func (p StoragePaths) DatedUploadKey(documentID string, subdoc *string, ts time.Time, storedName string) string {
	return path.Join(p.datedMediaKeyPrefix(documentID, subdoc, "files", ts), storedName)
}

func (p StoragePaths) DatedAudioDir(documentID string, subdoc *string, ts time.Time, ensure bool) string {
	return p.datedMediaDir(documentID, subdoc, "audio-notes", ts, ensure)
}

func (p StoragePaths) AudioKey(documentID string, subdoc *string, noteID string) string {
	base := path.Join("documents", documentID, "audio-notes")
	if subdoc != nil {
		base = path.Join(base, *subdoc)
	}
	return path.Join(base, noteID+".webm")
}

func (p StoragePaths) DatedAudioKey(documentID string, subdoc *string, ts time.Time, noteID string) string {
	return path.Join(p.datedMediaKeyPrefix(documentID, subdoc, "audio-notes", ts), noteID+".webm")
}

func (p StoragePaths) datedMediaDir(documentID string, subdoc *string, kind string, ts time.Time, ensure bool) string {
	base := p.DocumentDir(documentID)
	if subdoc != nil {
		base = filepath.Join(base, "subdocuments", *subdoc)
	}
	base = filepath.Join(base, kind, ts.Format("2006"), ts.Format("01"), ts.Format("02"))
	if ensure {
		_ = os.MkdirAll(base, 0o755)
	}
	return base
}

func (p StoragePaths) datedMediaKeyPrefix(documentID string, subdoc *string, kind string, ts time.Time) string {
	base := path.Join("documents", documentID)
	if subdoc != nil {
		base = path.Join(base, "subdocuments", *subdoc)
	}
	return path.Join(base, kind, ts.Format("2006"), ts.Format("01"), ts.Format("02"))
}

func (p StoragePaths) RelativeStoragePath(path string) string {
	rel, err := filepath.Rel(p.Root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func (p StoragePaths) ResolveStoragePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.Root, filepath.FromSlash(path))
}

func (p StoragePaths) FilesManifestPath(documentID string, subdoc *string) string {
	base := p.DocumentDir(documentID)
	if subdoc != nil {
		base = filepath.Join(base, *subdoc)
	}
	return filepath.Join(base, ".files-manifest.json")
}

func (p StoragePaths) FilesManifestKey(documentID string, subdoc *string) string {
	base := path.Join("documents", documentID)
	if subdoc != nil {
		base = path.Join(base, *subdoc)
	}
	return path.Join(base, ".files-manifest.json")
}

func (p StoragePaths) LegacyYSweetKey(documentID string) string {
	return path.Join(documentID, "data.ysweet")
}

func SubdocumentDBScope(subdoc *string) string {
	if subdoc == nil {
		return ""
	}
	return *subdoc
}
