package yupdate

import (
	"context"
	"errors"
	"fmt"
)

// detectAggregateUpdateFormatSkippingEmpty pré-valida o formato de uma lista de
// updates, ignorando payloads vazios e preservando o comportamento no-op quando
// toda a entrada está vazia.
func detectAggregateUpdateFormatSkippingEmpty(updates ...[]byte) (UpdateFormat, error) {
	return detectAggregateUpdateFormatSkippingEmptyContext(context.Background(), updates...)
}

func detectAggregateUpdateFormatSkippingEmptyContext(ctx context.Context, updates ...[]byte) (UpdateFormat, error) {
	format, err := DetectUpdatesFormatWithReasonContext(ctx, updates...)
	if err != nil {
		if errors.Is(err, ErrUnknownUpdateFormat) && !hasNonEmptyUpdate(updates) {
			return UpdateFormatUnknown, nil
		}
		return UpdateFormatUnknown, err
	}
	return format, nil
}

func hasNonEmptyUpdate(updates [][]byte) bool {
	for _, update := range updates {
		if len(update) != 0 {
			return true
		}
	}
	return false
}

func firstNonEmptyUpdateIndex(updates [][]byte) (int, bool) {
	for i, update := range updates {
		if len(update) != 0 {
			return i, true
		}
	}
	return 0, false
}

func wrapUnsupportedV2AtFirstNonEmpty(updates [][]byte) error {
	index, ok := firstNonEmptyUpdateIndex(updates)
	if !ok {
		return ErrUnsupportedUpdateFormatV2
	}
	return fmt.Errorf("update[%d]: %w", index, ErrUnsupportedUpdateFormatV2)
}
