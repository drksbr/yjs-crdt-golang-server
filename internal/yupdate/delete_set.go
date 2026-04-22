package yupdate

import "yjs-go-bridge/internal/ytypes"

func readDeleteSetV1(d *decoderV1) (*ytypes.DeleteSet, error) {
	return ReadDeleteSetBlockV1(d.reader)
}
