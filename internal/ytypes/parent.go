package ytypes

// ParentKind descreve como o parent foi referenciado no update.
type ParentKind uint8

const (
	ParentNone ParentKind = iota
	ParentID
	ParentRoot
)

// ParentRef representa a forma mínima de referência a parent antes de integração.
type ParentRef struct {
	root string
	id   *ID
}

// NewParentID cria uma referência a parent por ID.
func NewParentID(id ID) ParentRef {
	ref := id
	return ParentRef{id: &ref}
}

// NewParentRoot cria uma referência a top-level type por nome.
func NewParentRoot(root string) (ParentRef, error) {
	if root == "" {
		return ParentRef{}, ErrInvalidParentRoot
	}
	return ParentRef{root: root}, nil
}

// Kind informa qual variante da referência está preenchida.
func (p ParentRef) Kind() ParentKind {
	switch {
	case p.id != nil:
		return ParentID
	case p.root != "":
		return ParentRoot
	default:
		return ParentNone
	}
}

// IsZero informa se a struct não carrega parent explícito.
func (p ParentRef) IsZero() bool {
	return p.Kind() == ParentNone
}

// Root retorna o nome do root parent, quando houver.
func (p ParentRef) Root() string {
	return p.root
}

// ID retorna o identificador do parent, quando houver.
func (p ParentRef) ID() (ID, bool) {
	if p.id == nil {
		return ID{}, false
	}
	return *p.id, true
}
