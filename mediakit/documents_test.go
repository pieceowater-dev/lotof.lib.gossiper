package mediakit

import "testing"

func TestDetectDocument(t *testing.T) {
	zip := append([]byte("PK\x03\x04"), make([]byte, 20)...)
	ole := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 20)...)
	cases := []struct {
		name string
		file []byte
		ok   bool
	}{
		{"a.docx", zip, true}, {"A.XLSX", zip, true}, {"a.doc", ole, true}, {"a.xls", ole, true},
		{"a.txt", []byte("hello"), true}, {"a.csv", []byte("a,b\n1,2"), true},
		{"a.docx", []byte("<html>"), false}, {"a.doc", zip, false}, {"a.txt", []byte("a\x00b"), false},
		{"a.html", []byte("<html>"), false}, {"a.svg", []byte("<svg>"), false}, {"a.txt", nil, false}, {"noext", []byte("x"), false},
	}
	for _, c := range cases {
		_, _, err := DetectDocument(c.name, c.file)
		if (err == nil) != c.ok {
			t.Errorf("%s: ok=%v err=%v", c.name, c.ok, err)
		}
	}
	if !IsDocumentType("text/csv; charset=utf-8") || IsDocumentType("text/html") || DocumentType("x.PDF") != "" {
		t.Error("type lookup")
	}
}
