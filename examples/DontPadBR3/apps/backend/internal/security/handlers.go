package security

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/gin-gonic/gin"
)

func (s *Service) HandleGetToken(c *gin.Context) {
	access, err := s.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.HasAccess {
		common.WriteError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	token, err := s.SignToken(common.SignedToken{
		DocumentID: access.DocumentID,
		Scope:      "ws",
		ExpiresAt:  time.Now().Add(common.DefaultRealtimeTTL).Unix(),
	})
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Failed to create connection token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documentId": access.DocumentID,
		"canEdit":    access.CanEdit,
		"persist":    true,
		"wsBaseUrl":  s.WebsocketBaseURL(c.Request),
		"token":      token,
	})
}

func (s *Service) HandleGetSecurity(c *gin.Context) {
	access, err := s.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"isProtected":    access.IsProtected,
		"hasPin":         access.HasPIN,
		"hasAccess":      access.HasAccess,
		"visibilityMode": access.VisibilityMode,
		"canEdit":        access.CanEdit,
	})
}

func (s *Service) HandleSaveSecurity(c *gin.Context) {
	access, err := s.GetDocumentAccessState(c, c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !access.CanEdit {
		common.WriteError(c, http.StatusForbidden, "Forbidden")
		return
	}

	var req common.SecuritySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.WriteError(c, http.StatusBadRequest, "Body invalido")
		return
	}
	mode := req.VisibilityMode
	if !IsVisibilityModeValid(mode) {
		common.WriteError(c, http.StatusBadRequest, "visibilityMode invalido")
		return
	}

	current, err := s.loadSecurityRow(c.Request.Context(), access.DocumentID)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao carregar seguranca")
		return
	}

	var hash *string
	trimmedPIN := strings.TrimSpace(req.PIN)
	switch mode {
	case common.VisibilityPublic:
		hash = nil
	default:
		if trimmedPIN != "" {
			if len(trimmedPIN) < 4 {
				common.WriteError(c, http.StatusBadRequest, "PIN deve ter pelo menos 4 caracteres")
				return
			}
			value, err := hashPIN(trimmedPIN)
			if err != nil {
				common.WriteError(c, http.StatusInternalServerError, "Falha ao gerar hash do PIN")
				return
			}
			hash = &value
		} else if current.PinHash != nil && *current.PinHash != "" {
			hash = current.PinHash
		} else {
			common.WriteError(c, http.StatusBadRequest, "PIN obrigatorio para este modo")
			return
		}
	}

	if err := s.upsertSecurityRow(c.Request.Context(), access.DocumentID, mode, hash); err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Falha ao salvar seguranca")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Service) HandleVerifyPIN(c *gin.Context) {
	docID, err := common.NormalizeDocumentID(c.Param("documentId"))
	if err != nil {
		common.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}

	key := fmt.Sprintf("%s:%s", common.ClientIP(c.Request), docID)
	allowed, retryAfter := s.rateLimiter.allow(key, 10, 15*time.Minute)
	if !allowed {
		c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		common.WriteError(c, http.StatusTooManyRequests, "Muitas tentativas. Tente novamente mais tarde.")
		return
	}

	var req common.VerifyPinRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.PIN) == "" {
		common.WriteError(c, http.StatusBadRequest, "PIN é obrigatório")
		return
	}

	row, err := s.loadSecurityRow(c.Request.Context(), docID)
	if err != nil {
		common.WriteError(c, http.StatusInternalServerError, "Erro ao verificar PIN")
		return
	}
	if row.PinHash == nil || *row.PinHash == "" {
		common.WriteError(c, http.StatusBadRequest, "Documento não está protegido")
		return
	}

	valid := verifyPIN(req.PIN, *row.PinHash)
	if !valid && s.masterPassword != "" {
		valid = subtle.ConstantTimeCompare([]byte(req.PIN), []byte(s.masterPassword)) == 1
	}
	if !valid {
		common.WriteError(c, http.StatusUnauthorized, "PIN incorreto")
		return
	}

	s.rateLimiter.reset(key)
	s.SetDocumentAuthCookie(c, docID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}
