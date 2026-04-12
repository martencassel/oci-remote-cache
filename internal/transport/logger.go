package transport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cCyan   = "\033[36m"
	cGreen  = "\033[32m"
	cRed    = "\033[31m"
	cYellow = "\033[33m"
	cGray   = "\033[90m"
	cBlue   = "\033[34m"
)

func isTextContent(contentType string, body []byte) bool {
	if contentType == "" {
		// Heuristic: treat as text if body is UTF‑8 printable
		for _, b := range body {
			if b < 0x09 || (b > 0x0D && b < 0x20) {
				return false
			}
		}
		return true
	}

	mediatype, _, _ := mime.ParseMediaType(contentType)
	return strings.HasPrefix(mediatype, "text/") ||
		mediatype == "application/json" ||
		mediatype == "application/xml"
}

func summarizeBinary(body []byte) string {
	h := sha256.Sum256(body)
	return fmt.Sprintf(
		"<binary %d bytes, sha256=%s>",
		len(body),
		hex.EncodeToString(h[:]),
	)
}

func TraceHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// --- Capture request body ---
		var reqBody []byte
		// Do not touch no-body requests (e.g. GET/HEAD). Replacing Body with
		// io.NopCloser can make outbound ContentLength unknown (-1), which can
		// break 307 redirect following in net/http.
		hasRequestBody := c.Request.ContentLength != 0 || len(c.Request.TransferEncoding) > 0
		if c.Request.Body != nil && hasRequestBody {
			reqBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		}

		// --- Capture response body only for non-blob paths to avoid RAM blowup ---
		isBlobPath := strings.Contains(c.Request.URL.Path, "/blobs/")
		reqCT := c.GetHeader("Content-Type")

		// --- Capture request headers ---
		reqHeaders := make([]string, 0)
		for k, v := range c.Request.Header {
			reqHeaders = append(reqHeaders,
				fmt.Sprintf("    %s%s:%s %v", cGray, k, cReset, v),
			)
		}

		// --- Capture response body (skip for blobs to avoid buffering GBs) ---
		respBuf := new(bytes.Buffer)
		if !isBlobPath {
			writer := &bodyCaptureWriter{ResponseWriter: c.Writer, buf: respBuf}
			c.Writer = writer
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		respCT := c.Writer.Header().Get("Content-Type")

		// --- Color for status ---
		statusColor := cGreen
		if status >= 400 {
			statusColor = cRed
		}

		// --- Pretty response headers ---
		respHeaders := make([]string, 0)
		for k, v := range c.Writer.Header() {
			respHeaders = append(respHeaders,
				fmt.Sprintf("    %s%s:%s %v", cGray, k, cReset, v),
			)
		}

		// --- Body formatting ---
		var reqBodyOut, respBodyOut string

		if isTextContent(reqCT, reqBody) {
			reqBodyOut = string(reqBody)
		} else {
			reqBodyOut = summarizeBinary(reqBody)
		}

		respBody := respBuf.Bytes()
		if isTextContent(respCT, respBody) {
			respBodyOut = string(respBody)
		} else {
			respBodyOut = summarizeBinary(respBody)
		}

		// --- mitmproxy‑style output ---
		fmt.Printf(
			"\n%sREQUEST →%s %s%s %s%s\n"+
				"  %sHeaders:%s\n%s\n"+
				"  %sBody:%s %s\n\n"+
				"%sRESPONSE ←%s\n"+
				"  %sStatus:%s %s%d%s\n"+
				"  %sLatency:%s %s\n"+
				"  %sHeaders:%s\n%s\n"+
				"  %sBody:%s %s\n\n",
			cBlue, cReset, cCyan, c.Request.Method, cReset, " "+c.Request.URL.Path,
			cBold, cReset, strings.Join(reqHeaders, "\n"),
			cBold, cReset, cYellow+reqBodyOut+cReset,

			cBlue, cReset,
			cBold, cReset, statusColor, status, cReset,
			cBold, cReset, latency,
			cBold, cReset, strings.Join(respHeaders, "\n"),
			cBold, cReset, cYellow+respBodyOut+cReset,
		)
	}
}

type bodyCaptureWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}
