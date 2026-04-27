package yupdate

import "testing"

func TestValidateUpdateFormatWithReasonKeepsDetectedV2(t *testing.T) {
	t.Parallel()

	update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	got, err := ValidateUpdateFormatWithReason(update)
	if err != nil {
		t.Fatalf("ValidateUpdateFormatWithReason() unexpected error: %v", err)
	}
	if got != UpdateFormatV2 {
		t.Fatalf("ValidateUpdateFormatWithReason() = %s, want %s", got, UpdateFormatV2)
	}
}
