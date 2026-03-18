package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
)

const maxJSONRequestBodyBytes int64 = 10 << 20 // 10 MB

func decodeJSONBodyLimited(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestBodyBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
