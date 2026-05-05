package documents

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/gin-gonic/gin"
)

func (s *Service) HandleFlush(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	body := http.MaxBytesReader(c.Writer, c.Request.Body, common.MaxDocumentUpdateBytes+1)
	update, err := common.ReadLimitedPayload(body, common.MaxDocumentUpdateBytes)
	if errors.Is(err, common.ErrPayloadTooLarge) {
		common.WriteError(c, http.StatusRequestEntityTooLarge, "Yjs update exceeds maximum allowed size")
		return
	}
	if err != nil || len(update) == 0 {
		common.WriteError(c, http.StatusBadRequest, "Missing Yjs update payload")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), common.FlushPersistTimeout)
	defer cancel()

	if err := s.EnsureLegacyMigrated(ctx, access.DocumentID); err != nil {
		log.Printf("legacy ysweet migration before flush failed doc=%s err=%v", access.DocumentID, err)
		common.WriteError(c, http.StatusInternalServerError, "Failed to migrate legacy document state")
		return
	}

	if err := s.applyUpdateAndPersist(ctx, access.DocumentID, update); err != nil {
		log.Printf("flush failed doc=%s err=%v", access.DocumentID, err)
		common.WriteError(c, http.StatusInternalServerError, "Failed to persist document state")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "bytes": len(update)})
}

func (s *Service) HandleDeleteDocument(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	ctx := c.Request.Context()
	children, err := s.listDocumentChildren(ctx, access.DocumentID)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to load subdocument metadata")
		return
	}
	if err := s.deleteDocumentMetadata(ctx, access.DocumentID); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to delete document metadata")
		return
	}
	for _, child := range children {
		if err := s.deleteDocumentMetadata(ctx, child.DocumentID); err != nil {
			common.WriteError(c, http.StatusInternalServerError, "Failed to delete subdocument metadata")
			return
		}
		_ = os.RemoveAll(s.paths.DocumentDir(child.DocumentID))
	}

	docDir := s.paths.DocumentDir(access.DocumentID)
	_ = os.RemoveAll(docDir)
	s.security.ClearDocumentAuthCookie(c, access.DocumentID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Service) HandleListSubdocuments(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := s.EnsureLegacyMigrated(c.Request.Context(), access.DocumentID); err != nil {
		log.Printf("legacy ysweet migration before subdocument list failed doc=%s err=%v", access.DocumentID, err)
		common.WriteError(c, http.StatusInternalServerError, "Failed to migrate legacy document state")
		return
	}

	children, err := s.listDocumentChildren(c.Request.Context(), access.DocumentID)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to fetch subdocuments")
		return
	}
	c.JSON(http.StatusOK, gin.H{"subdocuments": children})
}

func (s *Service) HandleCreateSubdocument(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	var req common.DocumentChildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.WriteError(c, http.StatusBadRequest, "Body invalido")
		return
	}

	child, err := NewDocumentChild(access.DocumentID, req)
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.upsertDocumentChild(c.Request.Context(), &child); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to save subdocument")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"subdocument": child})
}

func (s *Service) HandleDeleteSubdocument(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	slug, err := common.NormalizeDocumentID(c.Param("slug"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, "Invalid subdocument slug")
		return
	}
	child, err := s.getDocumentChild(c.Request.Context(), access.DocumentID, slug)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			common.WriteError(c, http.StatusNotFound, "Subdocument not found")
			return
		}
		common.WriteError(c, http.StatusInternalServerError, "Failed to delete subdocument")
		return
	}
	if err := s.deleteDocumentChild(c.Request.Context(), access.DocumentID, slug); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to delete subdocument")
		return
	}
	if err := s.deleteDocumentMetadata(c.Request.Context(), child.DocumentID); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to delete subdocument metadata")
		return
	}
	_ = os.RemoveAll(s.paths.DocumentDir(child.DocumentID))
	c.JSON(http.StatusOK, gin.H{"success": true})
}
