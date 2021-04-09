package httpflv

import (
	"bytes"
	"fmt"
	"io"
)

type ChunkWriter struct {
	w     io.Writer
	cache *bytes.Buffer
}

func NewChunkWriter(w io.Writer) *ChunkWriter {
	return &ChunkWriter{
		w:     w,
		cache: &bytes.Buffer{},
	}
}

func (cw *ChunkWriter) Write(p []byte) (n int, err error) {
	length := len(p)
	if length == 0 {
		return 0, nil
	}

	strlen := fmt.Sprintf("%x\r\n", length)
	cw.w.Write([]byte(strlen))
	cw.w.Write(p)
	cw.w.Write([]byte("\r\n"))

	// cw.cache.Write(p)

	// for {
	// 	if cw.cache.Len() < 2048 {
	// 		break
	// 	}
	// 	fmt.Println("Debug..... ", cw.cache.Len())

	// 	rbuf := make([]byte, 2048)
	// 	cw.cache.Read(rbuf)

	// 	cw.w.Write([]byte("800\r\n"))
	// 	cw.w.Write(rbuf)
	// 	cw.w.Write([]byte("\r\n"))
	// }
	return length, nil
}
