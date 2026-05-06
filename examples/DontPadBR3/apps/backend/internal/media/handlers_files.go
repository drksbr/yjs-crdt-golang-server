package media

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/objectstore"
	"github.com/gin-gonic/gin"
)

func (s *Service) HandleListFiles(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.Query("subdocumentId"))
	files, err := s.listFileEntries(c.Request.Context(), access.DocumentID, subdoc)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to read files")
		return
	}
	c.JSON(http.StatusOK, files)
}

func (s *Service) HandleUploadFile(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.Query("subdocumentId"))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, "No file provided")
		return
	}
	if fileHeader.Size > common.MaxFileSizeBytes {
		common.WriteError(c, http.StatusBadRequest, "File size exceeds maximum limit of 50MB")
		return
	}

	fileExt := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileHeader.Filename)), ".")
	if fileExt == "" {
		common.WriteError(c, http.StatusBadRequest, "Tipo de arquivo não permitido. Confira os formatos aceitos.")
		return
	}
	if _, ok := common.AllowedFileExt[fileExt]; !ok {
		common.WriteError(c, http.StatusBadRequest, "Tipo de arquivo não permitido. Confira os formatos aceitos.")
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, "Falha ao abrir arquivo")
		return
	}
	defer func() {
		_ = src.Close()
	}()

	peek := make([]byte, 512)
	n, _ := io.ReadFull(src, peek)
	contentType := http.DetectContentType(peek[:n])
	if _, ok := common.AllowedFileMIMEs[contentType]; !ok {
		// fallback para content-type enviado pelo browser
		contentType = fileHeader.Header.Get("Content-Type")
		if _, ok := common.AllowedFileMIMEs[contentType]; !ok {
			common.WriteError(c, http.StatusBadRequest, "Tipo de arquivo não permitido. Confira os formatos aceitos.")
			return
		}
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}

	fileID, err := common.RandomUUID()
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao gerar id de arquivo")
		return
	}

	now := time.Now()
	storedName := fmt.Sprintf("%s.%s", fileID, fileExt)
	storagePath := s.paths.DatedUploadKey(access.DocumentID, subdoc, now, storedName)
	written, err := s.objects.Put(c.Request.Context(), storagePath, src, objectstore.PutOptions{
		ContentType: contentType,
		MaxBytes:    common.MaxFileSizeBytes,
	})
	if err != nil {
		if errors.Is(err, common.ErrPayloadTooLarge) {
			common.WriteError(c, http.StatusBadRequest, "File size exceeds maximum limit of 50MB")
			return
		}
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar arquivo")
		return
	}

	entry := common.DocumentFile{
		ID:           fileID,
		Name:         storedName,
		OriginalName: fileHeader.Filename,
		MimeType:     contentType,
		Size:         written,
		UploadedAt:   now.UnixMilli(),
		StoragePath:  storagePath,
	}
	if err := s.upsertFileEntry(c.Request.Context(), access.DocumentID, subdoc, entry); err != nil {
		_ = s.objects.Delete(c.Request.Context(), storagePath)
		common.WriteError(c, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	c.JSON(http.StatusCreated, entry)
}

func (s *Service) HandleDeleteFile(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	fileID := strings.ToLower(strings.TrimSpace(c.Query("fileId")))
	if !common.UUIDPattern.MatchString(fileID) {
		common.WriteError(c, http.StatusBadRequest, "Invalid document ID or file ID")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.Query("subdocumentId"))
	legacyFileName := strings.TrimSpace(c.Query("fileName"))

	entry, err := s.getFileEntry(c.Request.Context(), access.DocumentID, subdoc, fileID)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to delete file")
		return
	}
	if entry == nil && legacyFileName == "" {
		common.WriteError(c, http.StatusNotFound, "File not found")
		return
	}

	storedName := legacyFileName
	if entry != nil {
		storedName = entry.Name
	}
	if err := common.AssertStoredFileName(fileID, storedName); err != nil {
		common.WriteError(c, http.StatusBadRequest, "Invalid file name")
		return
	}

	storagePath := s.paths.UploadKey(access.DocumentID, subdoc, storedName)
	if entry != nil && entry.StoragePath != "" {
		storagePath = entry.StoragePath
	}
	_ = s.objects.Delete(c.Request.Context(), storagePath)
	if entry != nil {
		if err := s.removeFileEntry(c.Request.Context(), access.DocumentID, subdoc, fileID); err != nil {
			common.WriteError(c, http.StatusInternalServerError, "Failed to delete file")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "File deleted successfully"})
}

func (s *Service) HandleDownloadFile(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	fileID := strings.ToLower(strings.TrimSpace(c.Query("fileId")))
	if !common.UUIDPattern.MatchString(fileID) {
		common.WriteError(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.Query("subdocumentId"))
	legacyFileName := strings.TrimSpace(c.Query("fileName"))
	legacyOriginal := strings.TrimSpace(c.Query("originalName"))

	entry, err := s.getFileEntry(c.Request.Context(), access.DocumentID, subdoc, fileID)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to download file")
		return
	}
	if entry == nil && legacyFileName == "" {
		common.WriteError(c, http.StatusNotFound, "File not found")
		return
	}

	storedName := legacyFileName
	displayName := legacyOriginal
	if entry != nil {
		storedName = entry.Name
		displayName = entry.OriginalName
	}
	if displayName == "" {
		displayName = storedName
	}
	if err := common.AssertStoredFileName(fileID, storedName); err != nil {
		common.WriteError(c, http.StatusBadRequest, "Invalid file name")
		return
	}

	storagePath := s.paths.UploadKey(access.DocumentID, subdoc, storedName)
	if entry != nil && entry.StoragePath != "" {
		storagePath = entry.StoragePath
	}
	content, err := objectstore.ReadAll(c.Request.Context(), s.objects, storagePath, common.MaxFileSizeBytes)
	if err != nil {
		common.WriteError(c, http.StatusNotFound, "File not found")
		return
	}

	ascii := common.SanitizeASCII(displayName)
	encoded := url.QueryEscape(displayName)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", ascii, encoded))
	c.Header("Content-Length", strconv.FormatInt(int64(len(content)), 10))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/octet-stream", content)
}
