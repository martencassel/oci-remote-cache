package stream

import (
	"io"
	"os"
)

func Duplicate(upstream io.ReadCloser, cacheFile *os.File) io.ReadCloser {
	pr, pw := io.Pipe()
	tee := io.TeeReader(upstream, pw)
	go func() {
		_, err := io.Copy(cacheFile, tee)
		cacheFile.Close()
		upstream.Close()
		// Close pw last: this signals io.EOF (or the error) to whoever reads pr.
		// Closing pr first would fire the pipe's sync.Once and cause pr.Read to
		// return io.ErrClosedPipe instead of io.EOF, breaking callers.
		pw.CloseWithError(err)
	}()
	return pr
}
