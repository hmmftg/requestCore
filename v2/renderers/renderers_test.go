package renderers

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSONRenderer_ContentType(t *testing.T) {
	r := JSONRenderer{}
	if ct := r.ContentType(); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestJSONRenderer_Encode(t *testing.T) {
	r := JSONRenderer{}
	data := map[string]any{"name": "test", "count": 42}
	buf, err := r.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !strings.Contains(string(buf), `"name":"test"`) {
		t.Fatalf("expected name field in output, got %s", string(buf))
	}
}

func TestJSONRenderer_EncodeIndented(t *testing.T) {
	r := JSONRenderer{Indent: "  "}
	data := map[string]any{"name": "test"}
	buf, err := r.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !strings.Contains(string(buf), "\n") {
		t.Fatalf("expected indented output with newlines, got %s", string(buf))
	}
}

func TestJSONRenderer_EncodeNil(t *testing.T) {
	r := JSONRenderer{}
	buf, err := r.Encode(nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if string(buf) != "null" {
		t.Fatalf("expected null, got %s", string(buf))
	}
}

func TestXMLRenderer_ContentType(t *testing.T) {
	r := XMLRenderer{}
	if ct := r.ContentType(); ct != "application/xml" {
		t.Fatalf("expected application/xml, got %s", ct)
	}
}

func TestXMLRenderer_Encode(t *testing.T) {
	r := XMLRenderer{}
	type person struct {
		XMLName struct{} `xml:"person"`
		Name    string   `xml:"name"`
	}
	buf, err := r.Encode(person{Name: "test"})
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !strings.HasPrefix(string(buf), `<?xml`) {
		t.Fatalf("expected XML header, got %s", string(buf))
	}
	if !strings.Contains(string(buf), `<name>test</name>`) {
		t.Fatalf("expected name field, got %s", string(buf))
	}
}

func TestTextRenderer_ContentType(t *testing.T) {
	r := TextRenderer{}
	if ct := r.ContentType(); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain; charset=utf-8, got %s", ct)
	}
}

func TestTextRenderer_EncodeString(t *testing.T) {
	r := TextRenderer{}
	buf, err := r.Encode("hello world")
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if string(buf) != "hello world" {
		t.Fatalf("expected 'hello world', got %s", string(buf))
	}
}

func TestTextRenderer_EncodeBytes(t *testing.T) {
	r := TextRenderer{}
	buf, err := r.Encode([]byte("raw bytes"))
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !bytes.Equal(buf, []byte("raw bytes")) {
		t.Fatalf("expected 'raw bytes', got %s", string(buf))
	}
}

func TestTextRenderer_EncodeStringer(t *testing.T) {
	r := TextRenderer{}
	buf, err := r.Encode(t)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(buf) == 0 {
		t.Fatal("expected non-empty output from Stringer")
	}
}

func TestTextRenderer_EncodeDefault(t *testing.T) {
	r := TextRenderer{}
	buf, err := r.Encode(42)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if string(buf) != "42" {
		t.Fatalf("expected '42', got %s", string(buf))
	}
}

func TestCSVRenderer_ContentType(t *testing.T) {
	r := CSVRenderer{}
	if ct := r.ContentType(); ct != "text/csv" {
		t.Fatalf("expected text/csv, got %s", ct)
	}
}

func TestCSVRenderer_EncodeStringRows(t *testing.T) {
	r := CSVRenderer{}
	data := [][]string{
		{"name", "age"},
		{"Alice", "30"},
		{"Bob", "25"},
	}
	buf, err := r.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(buf)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), string(buf))
	}
	if !strings.Contains(lines[0], "name") {
		t.Fatalf("expected header with 'name', got %s", lines[0])
	}
}

func TestCSVRenderer_EncodeStructs(t *testing.T) {
	r := CSVRenderer{}
	type user struct {
		Name string `csv:"name"`
		Age  int    `csv:"age"`
	}
	data := []user{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}
	buf, err := r.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	output := string(buf)
	if !strings.Contains(output, "name") {
		t.Fatalf("expected header 'name', got %s", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Fatalf("expected 'Alice' in output, got %s", output)
	}
}

func TestCSVRenderer_EncodeStructsWithHeaders(t *testing.T) {
	type user struct {
		Name string `csv:"name"`
		Age  int    `csv:"age"`
		City string `csv:"city"`
	}
	data := []user{
		{Name: "Alice", Age: 30, City: "NYC"},
	}
	r := CSVRenderer{Headers: []string{"name", "city"}}
	buf, err := r.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(buf)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), string(buf))
	}
	if !strings.Contains(lines[0], "name") || !strings.Contains(lines[0], "city") {
		t.Fatalf("expected header with name and city, got %s", lines[0])
	}
	if strings.Contains(lines[0], "age") {
		t.Fatalf("did not expect 'age' in header, got %s", lines[0])
	}
}

func TestCSVRenderer_EncodeSingleValue(t *testing.T) {
	r := CSVRenderer{}
	buf, err := r.Encode(42)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !strings.Contains(string(buf), "42") {
		t.Fatalf("expected '42' in output, got %s", string(buf))
	}
}

func TestCSVRenderer_EncodeNonStructSlice(t *testing.T) {
	r := CSVRenderer{}
	data := []int{1, 2, 3}
	buf, err := r.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(buf)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), string(buf))
	}
}

func TestCSVRenderer_HeaderNotFound(t *testing.T) {
	type user struct {
		Name string `csv:"name"`
	}
	data := []user{{Name: "Alice"}}
	r := CSVRenderer{Headers: []string{"nonexistent"}}
	_, err := r.Encode(data)
	if err == nil {
		t.Fatal("expected error for nonexistent header")
	}
}
