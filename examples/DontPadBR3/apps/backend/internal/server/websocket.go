package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func (a *Server) resolveWSRequest(r *http.Request) (yhttp.Request, error) {
	query := r.URL.Query()

	rawDoc := strings.TrimSpace(query.Get("doc"))
	docID, err := common.NormalizeDocumentID(rawDoc)
	if err != nil {
		return yhttp.Request{}, err
	}

	clientRaw := strings.TrimSpace(query.Get("client"))
	if clientRaw == "" {
		return yhttp.Request{}, errors.New("client obrigatorio")
	}
	clientValue, err := strconv.ParseUint(clientRaw, 10, 32)
	if err != nil {
		return yhttp.Request{}, fmt.Errorf("client invalido: %w", err)
	}

	wsToken := strings.TrimSpace(query.Get("token"))
	if wsToken == "" {
		return yhttp.Request{}, errors.New("token obrigatorio")
	}
	token, err := a.security.VerifySignedToken(wsToken, "ws")
	if err != nil {
		return yhttp.Request{}, errors.New("token invalido")
	}
	if token.DocumentID != docID {
		return yhttp.Request{}, errors.New("token nao corresponde ao documento")
	}
	if err := a.documents.EnsureLegacyMigrated(r.Context(), docID); err != nil {
		return yhttp.Request{}, fmt.Errorf("migracao ysweet legada falhou: %w", err)
	}

	persistOnClose := true
	persistRaw := strings.TrimSpace(query.Get("persist"))
	if persistRaw != "" {
		persistOnClose = persistRaw == "1" || strings.EqualFold(persistRaw, "true")
	}

	syncOutputFormat, err := resolveDontPadSyncOutputFormat(r)
	if err != nil {
		return yhttp.Request{}, err
	}

	connID := strings.TrimSpace(query.Get("conn"))
	if connID == "" {
		connID = fmt.Sprintf("ws-%d", time.Now().UnixNano())
	}

	return yhttp.Request{
		DocumentKey: storage.DocumentKey{
			Namespace:  a.cfg.Namespace,
			DocumentID: docID,
		},
		ConnectionID:     connID,
		ClientID:         uint32(clientValue),
		PersistOnClose:   persistOnClose,
		SyncOutputFormat: syncOutputFormat,
	}, nil
}

func resolveDontPadSyncOutputFormat(r *http.Request) (yjsbridge.UpdateFormat, error) {
	if !hasExplicitSyncOutputFormat(r) {
		return yjsbridge.UpdateFormatV2, nil
	}
	return yhttp.SyncOutputFormatFromHTTPRequest(r)
}

func hasExplicitSyncOutputFormat(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.URL != nil && strings.TrimSpace(r.URL.Query().Get(yhttp.SyncOutputFormatQueryParam)) != "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get(yhttp.SyncOutputFormatHeader)) != "" {
		return true
	}
	for _, value := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if strings.EqualFold(token, yhttp.SyncOutputFormatSubprotocolV1) ||
				strings.EqualFold(token, yhttp.SyncOutputFormatSubprotocolV2) {
				return true
			}
		}
	}
	return false
}
