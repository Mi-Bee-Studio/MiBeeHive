package rulesrc

import (
	"encoding/json"
	"io"
)

// jsonDecode is a thin wrapper kept separate so the import is localized.
func jsonDecode(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	return dec.Decode(v)
}
