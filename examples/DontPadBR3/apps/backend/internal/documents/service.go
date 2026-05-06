package documents

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/objectstore"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/security"
	pgstore "github.com/drksbr/yjs-crdt-golang-server/pkg/storage/postgres"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db        *pgxpool.Pool
	schemaSQL string
	namespace string
	store     *pgstore.Store
	provider  *yprotocol.Provider
	paths     common.StoragePaths
	security  *security.Service
	objects   objectstore.Store

	nextClientID atomic.Uint32
	legacy       *LegacyYSweetMigrator
	legacyMu     sync.Mutex
	legacyLocks  map[string]*sync.Mutex
}

type Deps struct {
	DB        *pgxpool.Pool
	SchemaSQL string
	Namespace string
	Store     *pgstore.Store
	Provider  *yprotocol.Provider
	Paths     common.StoragePaths
	Security  *security.Service
	Objects   objectstore.Store
	Legacy    *LegacyYSweetMigrator
}

func New(deps Deps) *Service {
	svc := &Service{
		db:          deps.DB,
		schemaSQL:   deps.SchemaSQL,
		namespace:   deps.Namespace,
		store:       deps.Store,
		provider:    deps.Provider,
		paths:       deps.Paths,
		security:    deps.Security,
		objects:     deps.Objects,
		legacy:      deps.Legacy,
		legacyLocks: make(map[string]*sync.Mutex),
	}
	svc.nextClientID.Store(uint32(time.Now().UnixNano()))
	return svc
}
