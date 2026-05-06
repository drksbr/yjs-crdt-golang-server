package media

import (
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/objectstore"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/security"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db        *pgxpool.Pool
	schemaSQL string
	namespace string
	paths     common.StoragePaths
	security  *security.Service
	objects   objectstore.Store
}

type Deps struct {
	DB        *pgxpool.Pool
	SchemaSQL string
	Namespace string
	Paths     common.StoragePaths
	Security  *security.Service
	Objects   objectstore.Store
}

func New(deps Deps) *Service {
	return &Service{
		db:        deps.DB,
		schemaSQL: deps.SchemaSQL,
		namespace: deps.Namespace,
		paths:     deps.Paths,
		security:  deps.Security,
		objects:   deps.Objects,
	}
}
