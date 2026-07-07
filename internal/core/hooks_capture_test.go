package core

import "testing"

func TestCappedWriterKeepsTail(t *testing.T) {
	w := &cappedWriter{limit: 5}
	if _, err := w.Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	if w.String() != "defgh" {
		t.Errorf("cappedWriter tail = %q, want defgh", w.String())
	}
}
