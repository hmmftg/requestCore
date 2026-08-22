package renderers

import "encoding/xml"

// XMLRenderer encodes data as XML.
type XMLRenderer struct {
	// Indent, when non-empty, enables indented output.
	Indent string
}

// ContentType returns the XML content type.
func (r XMLRenderer) ContentType() string {
	return "application/xml"
}

// Encode serializes data as XML, prefixed with the standard XML header.
func (r XMLRenderer) Encode(data any) ([]byte, error) {
	if r.Indent != "" {
		buf, err := xml.MarshalIndent(data, "", r.Indent)
		if err != nil {
			return nil, err
		}
		return append([]byte(xml.Header), buf...), nil
	}
	buf, err := xml.Marshal(data)
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), buf...), nil
}
