package pclinkcsv

import (
	"errors"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

func readAllBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func decodeUTF8(b []byte) (string, error) {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		b = b[3:]
	}
	if !utf8.Valid(b) {
		return "", errors.New("not valid utf-8")
	}
	return string(b), nil
}

func decodeCP932(b []byte) (string, error) {
	out, err := japanese.ShiftJIS.NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
