package yupdate

import "fmt"

const maxDecodedCollectionLength uint32 = 1 << 20

func validateDecodedCollectionLength(op string, offset int, length uint32) error {
	if length <= maxDecodedCollectionLength {
		return nil
	}
	return wrapError(op, offset, fmt.Errorf("%w: %d > %d", ErrDecodedCollectionTooLarge, length, maxDecodedCollectionLength))
}
