package citransport

import (
	"bytes"
	"compress/gzip"
	"sync"
)

// Compressor reuse avoids allocating flate's tables for every CI batch. Large
// output buffers are discarded so one unusually large payload is not retained.
type gzipCompressor struct {
	buffer bytes.Buffer
	writer *gzip.Writer
}

var gzipCompressors = sync.Pool{New: func() any {
	compressor := &gzipCompressor{}
	compressor.writer = gzip.NewWriter(&compressor.buffer)
	return compressor
}}

func releaseCompressor(compressor *gzipCompressor) {
	if compressor.buffer.Cap() > TestCycleFlushBytes {
		compressor.buffer = bytes.Buffer{}
	} else {
		compressor.buffer.Reset()
	}
	gzipCompressors.Put(compressor)
}
