package media

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/gin-gonic/gin"
)

func (s *Service) HandleListAudioNotes(c *gin.Context) {
	rawDoc := c.Query("documentId")
	if strings.TrimSpace(rawDoc) == "" {
		common.WriteError(c, http.StatusBadRequest, "documentId é obrigatório")
		return
	}

	access, err := s.security.GetDocumentAccessState(c, rawDoc)
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.Query("subdocumentId"))
	notes, err := s.listAudioNotes(c.Request.Context(), access.DocumentID, subdoc)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to read audio notes")
		return
	}
	c.JSON(http.StatusOK, notes)
}

func (s *Service) HandleCreateAudioNote(c *gin.Context) {
	audioHeader, err := c.FormFile("audio")
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, "Audio e documentId são obrigatórios")
		return
	}
	rawDoc := c.PostForm("documentId")
	if strings.TrimSpace(rawDoc) == "" {
		common.WriteError(c, http.StatusBadRequest, "Audio e documentId são obrigatórios")
		return
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(c.PostForm("duration")), 64)
	if err != nil || duration < 0 {
		common.WriteError(c, http.StatusBadRequest, "Duração inválida")
		return
	}

	access, err := s.security.GetDocumentAccessState(c, rawDoc)
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	if audioHeader.Size > common.MaxAudioSizeBytes {
		common.WriteError(c, http.StatusBadRequest, "Nota de áudio excede o limite permitido")
		return
	}
	contentType := audioHeader.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "audio/webm") {
		common.WriteError(c, http.StatusBadRequest, "Formato de áudio não permitido")
		return
	}

	src, err := audioHeader.Open()
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar nota de áudio")
		return
	}
	defer func() {
		_ = src.Close()
	}()

	noteID, err := common.RandomUUID()
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar nota de áudio")
		return
	}

	now := time.Now()
	subdoc := common.NormalizeOptionalSubdocumentID(c.PostForm("subdocumentId"))
	audioDir := s.paths.DatedAudioDir(access.DocumentID, subdoc, now, true)
	targetPath := filepath.Join(audioDir, noteID+".webm")
	storagePath := s.paths.RelativeStoragePath(targetPath)
	target, err := os.Create(targetPath)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar nota de áudio")
		return
	}
	defer func() {
		_ = target.Close()
	}()

	written, err := io.Copy(target, io.LimitReader(src, common.MaxAudioSizeBytes+1))
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar nota de áudio")
		return
	}
	if written > common.MaxAudioSizeBytes {
		_ = os.Remove(targetPath)
		common.WriteError(c, http.StatusBadRequest, "Nota de áudio excede o limite permitido")
		return
	}

	note := common.AudioNote{
		ID:          noteID,
		Name:        fmt.Sprintf("Nota de Áudio %s", now.Format("02/01/2006 15:04")),
		Duration:    common.MathRound(duration, 1),
		MimeType:    "audio/webm",
		Size:        written,
		CreatedAt:   now.UnixMilli(),
		StoragePath: storagePath,
	}
	if err := s.upsertAudioNote(c.Request.Context(), access.DocumentID, subdoc, note); err != nil {
		_ = os.Remove(targetPath)
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar nota de áudio")
		return
	}

	c.JSON(http.StatusOK, note)
}

func (s *Service) HandleDeleteAudioNote(c *gin.Context) {
	var req common.AudioDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.WriteError(c, http.StatusBadRequest, "documentId e noteId são obrigatórios")
		return
	}
	if strings.TrimSpace(req.DocumentID) == "" || strings.TrimSpace(req.NoteID) == "" {
		common.WriteError(c, http.StatusBadRequest, "documentId e noteId são obrigatórios")
		return
	}

	access, err := s.security.GetDocumentAccessState(c, req.DocumentID)
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}
	if !common.UUIDPattern.MatchString(strings.ToLower(req.NoteID)) {
		common.WriteError(c, http.StatusBadRequest, "noteId invalido")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(req.Subdocument)
	noteID := strings.ToLower(req.NoteID)
	path := filepath.Join(s.paths.AudioDir(access.DocumentID, subdoc, true), noteID+".webm")
	if note, err := s.getAudioNote(c.Request.Context(), access.DocumentID, subdoc, noteID); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao deletar áudio")
		return
	} else if note != nil && note.StoragePath != "" {
		path = s.paths.ResolveStoragePath(note.StoragePath)
	}
	_ = os.Remove(path)
	if err := s.removeAudioNote(c.Request.Context(), access.DocumentID, subdoc, noteID); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao deletar áudio")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Service) HandleGetAudioNote(c *gin.Context) {
	noteID := strings.ToLower(strings.TrimSpace(c.Param("noteId")))
	if !common.UUIDPattern.MatchString(noteID) {
		common.WriteError(c, http.StatusBadRequest, "documentId e noteId são obrigatórios")
		return
	}

	rawDoc := c.GetHeader("X-Document-Id")
	if strings.TrimSpace(rawDoc) == "" {
		common.WriteError(c, http.StatusBadRequest, "documentId e noteId são obrigatórios")
		return
	}

	access, err := s.security.GetDocumentAccessState(c, rawDoc)
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.GetHeader("X-Subdocument-Id"))
	path := filepath.Join(s.paths.AudioDir(access.DocumentID, subdoc, false), noteID+".webm")
	mimeType := "audio/webm"
	if note, err := s.getAudioNote(c.Request.Context(), access.DocumentID, subdoc, noteID); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao carregar áudio")
		return
	} else if note != nil {
		if note.StoragePath != "" {
			path = s.paths.ResolveStoragePath(note.StoragePath)
		}
		if note.MimeType != "" {
			mimeType = note.MimeType
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		common.WriteError(c, http.StatusNotFound, "Nota de áudio não encontrada")
		return
	}

	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"audio-%s.webm\"", noteID))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, mimeType, content)
}
