package yupdate

import (
	"context"
	"errors"
)

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
