package renderers

import "encoding/json"

// JSONRenderer encodes data as JSON.
type JSONRenderer struct {
	// Indent, when non-empty, enables indented output using the given
	// string as the indentation prefix (e.g. "  " for two spaces).
	Indent string
}

// ContentType returns the JSON content type.
func (r JSONRenderer) ContentType() string {
	return "application/json"
}

// Encode serializes data as JSON.
func (r JSONRenderer) Encode(data any) ([]byte, error) {
	if r.Indent != "" {
		buf, err := json.MarshalIndent(data, "", r.Indent)
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
	return json.Marshal(data)
}
