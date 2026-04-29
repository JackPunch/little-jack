package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// writeDebugBody writes a payload to debug output. If it is valid JSON,
// it is indented for readability; otherwise it is written as-is.
func writeDebugBody(w io.Writer, body []byte) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err == nil {
		fmt.Fprintf(w, "%s\n", buf.String())
	} else {
		fmt.Fprintf(w, "%s\n", string(body))
	}
}

// writeDebugSSE writes an SSE data line to debug output. If the payload
// after "data: " is valid JSON, it prints "data:" on its own line followed
// by the indented JSON; otherwise it writes the line as-is.
func writeDebugSSE(w io.Writer, line string) {
	data := strings.TrimPrefix(line, "data: ")
	if data == line { // not a data: line
		fmt.Fprintf(w, "%s", line)
		return
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(data), "", "  "); err == nil {
		fmt.Fprintf(w, "data:\n%s\n", buf.String())
	} else {
		fmt.Fprintf(w, "%s", line)
	}
}

// normalizeBaseURL ensures the base URL has a scheme (https by default).
func normalizeBaseURL(base string) string {
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		return base
	}
	return "https://" + base
}

func doRequest(req *http.Request, debugOut io.Writer) ([]byte, error) {
	if debugOut != nil {
		fmt.Fprintf(debugOut, "=== API Request ===\n")
		fmt.Fprintf(debugOut, "%s %s\n", req.Method, req.URL)
		for k, v := range req.Header {
			fmt.Fprintf(debugOut, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		if req.Body != nil {
			bodyBytes, _ := io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			fmt.Fprintln(debugOut, "Body:")
			writeDebugBody(debugOut, bodyBytes)
		}
		fmt.Fprintf(debugOut, "===================\n\n")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if debugOut != nil {
		fmt.Fprintf(debugOut, "=== API Response ===\n")
		fmt.Fprintf(debugOut, "Status: %s\n", resp.Status)
		for k, v := range resp.Header {
			fmt.Fprintf(debugOut, "Header: %s: %s\n", k, strings.Join(v, ", "))
		}
		fmt.Fprintln(debugOut, "Body:")
		writeDebugBody(debugOut, body)
		fmt.Fprintf(debugOut, "====================\n")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
