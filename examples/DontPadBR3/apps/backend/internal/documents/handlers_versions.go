package documents

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/gin-gonic/gin"
)

func (s *Service) HandleListVersions(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	subdoc := common.NormalizeOptionalSubdocumentID(c.Query("subdocument"))
	versions, err := s.listVersions(c.Request.Context(), access.DocumentID, subdoc)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to fetch versions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (s *Service) HandleCreateVersion(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	if header := c.GetHeader("Content-Length"); header != "" {
		contentLength, err := strconv.ParseInt(header, 10, 64)
		if err == nil && contentLength > common.MaxMultipartFormBytes {
			common.WriteError(c, http.StatusRequestEntityTooLarge, "Version payload exceeds maximum allowed size")
			return
		}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, common.MaxMultipartFormBytes)
	if err := c.Request.ParseMultipartForm(common.MaxMultipartMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			common.WriteError(c, http.StatusRequestEntityTooLarge, "Version payload exceeds maximum allowed size")
			return
		}
		common.WriteError(c, http.StatusBadRequest, "Form data invalido")
		return
	}

	file, header, err := c.Request.FormFile("update")
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, "Missing 'update' field with document state")
		return
	}
	defer func() {
		_ = file.Close()
	}()

	if header.Size > common.MaxDocumentUpdateBytes {
		common.WriteError(c, http.StatusRequestEntityTooLarge, "Version update exceeds maximum allowed size")
		return
	}

	update, err := common.ReadLimitedPayload(file, common.MaxDocumentUpdateBytes)
	if errors.Is(err, common.ErrPayloadTooLarge) {
		common.WriteError(c, http.StatusRequestEntityTooLarge, "Version update exceeds maximum allowed size")
		return
	}
	if err != nil || len(update) == 0 {
		common.WriteError(c, http.StatusBadRequest, "Invalid update payload")
		return
	}

	label := common.OptionalString(c.PostForm("label"))
	subdoc := common.NormalizeOptionalSubdocumentID(c.PostForm("subdocument"))
	createdBy := common.OptionalString(c.PostForm("createdBy"))
	versionID := common.GenerateVersionID()
	now := time.Now()

	version := common.DocumentVersion{
		ID:              versionID,
		DocumentID:      access.DocumentID,
		SubdocumentName: subdoc,
		Timestamp:       now.UnixMilli(),
		Label:           label,
		Size:            int64(len(update)),
		CreatedBy:       createdBy,
	}

	if err := s.insertVersion(c.Request.Context(), version, update, int64(len(update))); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to create version")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"version": version})
}

func (s *Service) HandleGetVersion(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	versionID := c.Param("versionId")
	if !common.VersionIDPattern.MatchString(versionID) {
		common.WriteError(c, http.StatusBadRequest, "Invalid version ID")
		return
	}

	includeData := c.Query("includeData") == "true"
	version, data, err := s.getVersion(c.Request.Context(), access.DocumentID, versionID, includeData)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			common.WriteError(c, http.StatusNotFound, "Version not found")
			return
		}
		common.WriteError(c, http.StatusInternalServerError, "Failed to fetch version")
		return
	}

	if includeData {
		ascii := common.SanitizeASCII(version.ID + ".yupdate")
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", ascii))
		c.Header("Cache-Control", "private, no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Version-Id", version.ID)
		c.Header("X-Version-Timestamp", strconv.FormatInt(version.Timestamp, 10))
		if version.Label != nil {
			c.Header("X-Version-Label", *version.Label)
		}
		c.Header("X-Version-Size", strconv.FormatInt(version.Size, 10))
		c.Data(http.StatusOK, "application/octet-stream", data)
		return
	}

	c.JSON(http.StatusOK, gin.H{"version": version})
}

func (s *Service) HandleRestoreVersion(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	versionID := c.Param("versionId")
	if !common.VersionIDPattern.MatchString(versionID) {
		common.WriteError(c, http.StatusBadRequest, "Invalid version ID")
		return
	}

	_, data, err := s.getVersion(c.Request.Context(), access.DocumentID, versionID, true)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			common.WriteError(c, http.StatusNotFound, "Version not found")
			return
		}
		common.WriteError(c, http.StatusInternalServerError, "Failed to restore version")
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (s *Service) HandleDeleteVersion(c *gin.Context) {
	access, err := s.security.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	versionID := c.Param("versionId")
	if !common.VersionIDPattern.MatchString(versionID) {
		common.WriteError(c, http.StatusBadRequest, "Invalid version ID")
		return
	}

	if err := s.deleteVersion(c.Request.Context(), access.DocumentID, versionID); err != nil {
		if errors.Is(err, common.ErrNotFound) {
			common.WriteError(c, http.StatusNotFound, "Version not found")
			return
		}
		common.WriteError(c, http.StatusInternalServerError, "Failed to delete version")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
