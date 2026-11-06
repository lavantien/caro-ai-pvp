package book

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoadRejectsMalformedMetaLines(t *testing.T) {
	cases := []struct {
		name string
		meta string
		want string
	}{
		{"empty meta", "", ""},
		{"array meta", "[1,2]\n", "not a json object"},
		{"number meta", "5\n", "not a json object"},
		{"duplicate field", `{"a":1,"a":2}` + "\n", `repeats field "a"`},
		{"truncated value", `{"a":` + "\n", ""},
		{"missing close brace", `{"a":1` + "\n", ""},
		{"content after object", `{"a":1} junk` + "\n", "content after the object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := specBook([]byte(tc.meta))
			_, err := Load(bytes.NewReader(data))
			if err == nil {
				t.Fatalf("Load accepted meta %q", tc.meta)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load(meta %q) = %v, want error containing %q", tc.meta, err, tc.want)
			}
		})
	}
}
