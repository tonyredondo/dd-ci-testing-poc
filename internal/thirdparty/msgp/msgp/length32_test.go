package msgp

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// Objects whose 32-bit length an int cannot hold on 32-bit platforms read
// as short bytes, as on 64-bit platforms, since no slice holds them, and
// skipping or copying one from a stream fails. Nothing panics or reads them
// with a wrapped length.
func TestLengthsBeyondInt(t *testing.T) {
	for _, length := range []uint32{1 << 31, 1<<32 - 1} {
		header := func(lead byte) []byte {
			return []byte{lead, byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
		}
		bin, str, array, object := header(mbin32), header(mstr32), header(marray32), header(mmap32)
		ext := append(header(mext32), 1, 0)
		for name, read := range map[string]func() error{
			"ReadBytesBytes": func() error { _, _, err := ReadBytesBytes(append(bin, 0), nil); return err },
			"ReadBytesZC":    func() error { _, _, err := ReadBytesZC(append(bin, 0)); return err },
			"ReadStringZC":   func() error { _, _, err := ReadStringZC(append(str, 0)); return err },
			"ReadStringBytes": func() error {
				_, _, err := ReadStringBytes(append(str, 0))
				return err
			},
			"ReadExtensionBytes": func() error {
				_, err := ReadExtensionBytes(ext, &RawExtension{Type: 1})
				return err
			},
			"Skip bin":   func() error { _, err := Skip(bin); return err },
			"Skip ext":   func() error { _, err := Skip(ext); return err },
			"Skip array": func() error { _, err := Skip(array); return err },
			"Skip map":   func() error { _, err := Skip(object); return err },
		} {
			if err := read(); !errors.Is(err, ErrShortBytes) {
				t.Errorf("%s of length %d: %v, want ErrShortBytes", name, length, err)
			}
		}
		for _, b := range [][]byte{bin, ext, object} {
			// Without io.Seeker, the reader cannot skip past the end.
			reader := func() *Reader { return NewReader(struct{ io.Reader }{bytes.NewReader(b)}) }
			if err := reader().Skip(); err == nil {
				t.Errorf("Reader.Skip of %x: no error", b)
			}
			if _, err := reader().CopyNext(io.Discard); err == nil {
				t.Errorf("Reader.CopyNext of %x: no error", b)
			}
		}
	}
}
