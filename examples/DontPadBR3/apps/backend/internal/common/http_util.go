package common

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func WriteError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message, "success": false})
}

func ReadLimitedPayload(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, ErrPayloadTooLarge
		}
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrPayloadTooLarge
	}
	return data, nil
}
