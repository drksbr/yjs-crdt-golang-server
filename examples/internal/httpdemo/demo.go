package httpdemo

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yhttp"
	"yjs-go-bridge/pkg/yprotocol"
)

const (
	Address = "127.0.0.1:8080"
	WSPath  = "/ws"
)

// NewMemoryHandler cria um handler WebSocket em cima de provider local e store
// em memória para uso nos examples de frameworks HTTP.
func NewMemoryHandler() (*yhttp.Server, error) {
	store := memory.New()
	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})

	return yhttp.NewServer(yhttp.ServerConfig{
		Provider:       provider,
		ResolveRequest: ResolveRequest,
	})
}

// ResolveRequest converte query string em configuração mínima para o handler.
func ResolveRequest(r *http.Request) (yhttp.Request, error) {
	query := r.URL.Query()
	documentID := strings.TrimSpace(query.Get("doc"))
	if documentID == "" {
		return yhttp.Request{}, errors.New("doc obrigatorio")
	}

	clientRaw := strings.TrimSpace(query.Get("client"))
	if clientRaw == "" {
		return yhttp.Request{}, errors.New("client obrigatorio")
	}

	clientValue, err := strconv.ParseUint(clientRaw, 10, 32)
	if err != nil {
		return yhttp.Request{}, err
	}

	return yhttp.Request{
		DocumentKey: storage.DocumentKey{
			Namespace:  "examples",
			DocumentID: documentID,
		},
		ConnectionID:   strings.TrimSpace(query.Get("conn")),
		ClientID:       uint32(clientValue),
		PersistOnClose: query.Get("persist") == "1",
	}, nil
}

// RootMessage retorna uma mensagem simples com o endpoint WebSocket esperado.
func RootMessage(label string) string {
	return fmt.Sprintf("%s: use ws://%s%s?doc=notes&client=101&persist=1\n", label, Address, WSPath)
}
