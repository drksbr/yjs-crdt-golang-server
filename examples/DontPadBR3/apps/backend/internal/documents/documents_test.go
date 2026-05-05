package documents

import (
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func TestNewDocumentChildUsesStableDocumentID(t *testing.T) {
	t.Parallel()

	left, err := NewDocumentChild("parent-pad", common.DocumentChildRequest{Name: "Checklist Geral", Type: "checklist"})
	if err != nil {
		t.Fatalf("NewDocumentChild() unexpected error: %v", err)
	}
	right, err := NewDocumentChild("parent-pad", common.DocumentChildRequest{Name: "Checklist Geral", Type: "checklist"})
	if err != nil {
		t.Fatalf("NewDocumentChild() unexpected error: %v", err)
	}
	if left.DocumentID != right.DocumentID {
		t.Fatalf("DocumentID is not stable: %q != %q", left.DocumentID, right.DocumentID)
	}
	if left.DocumentID == "parent-pad" || left.DocumentID == left.Slug {
		t.Fatalf("DocumentID = %q, want independent child document id", left.DocumentID)
	}
	if left.Slug != "checklist-geral" {
		t.Fatalf("Slug = %q, want checklist-geral", left.Slug)
	}
}

func TestListVersionsQueryUsesCorrectLimitPlaceholder(t *testing.T) {
	t.Parallel()

	svc := &Service{schemaSQL: `"dontpad_test"`, namespace: "tests"}

	query, args := svc.ListVersionsQuery("doc-root", nil)
	if !strings.Contains(query, "LIMIT $3") {
		t.Fatalf("ListVersionsQuery(nil subdoc) query = %q, want LIMIT $3", query)
	}
	if strings.Contains(query, "$4") {
		t.Fatalf("ListVersionsQuery(nil subdoc) query = %q, should not reference $4", query)
	}
	if len(args) != 3 {
		t.Fatalf("ListVersionsQuery(nil subdoc) args len = %d, want 3", len(args))
	}

	subdoc := "child-a"
	query, args = svc.ListVersionsQuery("doc-root", &subdoc)
	if !strings.Contains(query, "subdocument_name=$3") || !strings.Contains(query, "LIMIT $4") {
		t.Fatalf("ListVersionsQuery(subdoc) query = %q, want subdocument_name=$3 and LIMIT $4", query)
	}
	if len(args) != 4 {
		t.Fatalf("ListVersionsQuery(subdoc) args len = %d, want 4", len(args))
	}
}
