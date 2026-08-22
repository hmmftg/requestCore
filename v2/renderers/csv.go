package renderers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"reflect"
	"strings"
)

// CSVRenderer encodes data as CSV.
//
// Accepted input types:
//   - [][]string: each inner slice is a CSV row
//   - []T where T is a struct: struct fields become columns
//   - any other type: formatted as a single-cell row via fmt.Sprint
//
// When encoding structs, exported fields are used as columns.
// The optional Headers field selects and reorders columns by field name.
// If Headers is empty, all exported fields are included in declaration order.
type CSVRenderer struct {
	// Headers selects which struct fields to include and in what order.
	// When empty, all exported fields are used in declaration order.
	// Only applicable when encoding a slice of structs.
	Headers []string
}

// ContentType returns the CSV content type.
func (r CSVRenderer) ContentType() string {
	return "text/csv"
}

// Encode serializes data as CSV.
func (r CSVRenderer) Encode(data any) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	switch v := data.(type) {
	case [][]string:
		for _, row := range v {
			if err := w.Write(row); err != nil {
				return nil, fmt.Errorf("csv: write row: %w", err)
			}
		}
		w.Flush()
		return buf.Bytes(), nil
	default:
		sliceVal := reflect.ValueOf(data)
		if sliceVal.Kind() != reflect.Slice {
			if err := w.Write([]string{fmt.Sprint(data)}); err != nil {
				return nil, fmt.Errorf("csv: write single cell: %w", err)
			}
			w.Flush()
			return buf.Bytes(), nil
		}

		elemType := sliceVal.Type().Elem()
		if elemType.Kind() != reflect.Struct {
			for i := 0; i < sliceVal.Len(); i++ {
				if err := w.Write([]string{fmt.Sprint(sliceVal.Index(i).Interface())}); err != nil {
					return nil, fmt.Errorf("csv: write row %d: %w", i, err)
				}
			}
			w.Flush()
			return buf.Bytes(), nil
		}

		fields, err := selectStructFields(elemType, r.Headers)
		if err != nil {
			return nil, err
		}

		headerRow := make([]string, len(fields))
		for i, f := range fields {
			headerRow[i] = f.Name
		}
		if err := w.Write(headerRow); err != nil {
			return nil, fmt.Errorf("csv: write header: %w", err)
		}

		for i := 0; i < sliceVal.Len(); i++ {
			row := make([]string, len(fields))
			for j, f := range fields {
				fieldVal := sliceVal.Index(i).FieldByIndex(f.Index)
				row[j] = formatCSVField(fieldVal)
			}
			if err := w.Write(row); err != nil {
				return nil, fmt.Errorf("csv: write row %d: %w", i, err)
			}
		}
		w.Flush()
		return buf.Bytes(), nil
	}
}

type csvField struct {
	Name  string
	Index []int
}

func selectStructFields(elemType reflect.Type, headers []string) ([]csvField, error) {
	if len(headers) == 0 {
		var fields []csvField
		for i := 0; i < elemType.NumField(); i++ {
			f := elemType.Field(i)
			if !f.IsExported() {
				continue
			}
			name := f.Name
			if tag := f.Tag.Get("csv"); tag != "" {
				name = strings.Split(tag, ",")[0]
			}
			fields = append(fields, csvField{Name: name, Index: f.Index})
		}
		return fields, nil
	}

	fieldMap := make(map[string]csvField)
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Name
		if tag := f.Tag.Get("csv"); tag != "" {
			name = strings.Split(tag, ",")[0]
		}
		fieldMap[name] = csvField{Name: name, Index: f.Index}
		fieldMap[f.Name] = csvField{Name: name, Index: f.Index}
	}

	result := make([]csvField, 0, len(headers))
	for _, h := range headers {
		f, ok := fieldMap[h]
		if !ok {
			return nil, fmt.Errorf("csv: header %q not found in struct %s", h, elemType.Name())
		}
		result = append(result, f)
	}
	return result, nil
}

func formatCSVField(v reflect.Value) string {
	iface := v.Interface()
	if s, ok := iface.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprint(iface)
}
