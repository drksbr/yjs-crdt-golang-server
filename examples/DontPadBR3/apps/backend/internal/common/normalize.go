package common

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

func NormalizeDocumentID(raw string) (string, error) {
	value, err := url.PathUnescape(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("invalid document ID")
	}
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "doc_")
	value = DocUnsafePattern.ReplaceAllString(value, "")
	value = strings.Join(strings.Fields(value), "-")
	value = strings.Trim(value, "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	if value == "" {
		return "", errors.New("invalid document ID")
	}
	return value, nil
}

func NormalizeOptionalSubdocumentID(raw string) *string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return nil
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" || decoded == "." || decoded == ".." {
		return nil
	}
	if strings.ContainsAny(decoded, `/\`+"\x00") {
		return nil
	}
	return &decoded
}

func OptionalString(value string) *string {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	return &v
}

func AssertStoredFileName(fileID, fileName string) error {
	if !UUIDPattern.MatchString(strings.ToLower(fileID)) {
		return errors.New("invalid file ID")
	}
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(strings.ToLower(fileID)) + `\.[a-z0-9]{1,16}$`)
	if !pattern.MatchString(strings.ToLower(fileName)) {
		return errors.New("invalid file name")
	}
	return nil
}
