// Package pipe implements the UniCLI pipeline protocol for chaining tools.
//
// The protocol uses newline-delimited JSON (NDJSON) frames — one JSON object
// per line — for simplicity and debugability. Each frame carries metadata
// (sequence number, content type) and the payload data.
//
// Frame format (one per line):
//   {"seq":1,"type":"text","payload":"hello world\n"}
//   {"seq":2,"type":"eos"}
//
// Content types: text, json, binary, eos (end of stream)
//
// This can be upgraded to protobuf varint-length-prefixed framing later
// without changing the API surface.
package pipe

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ContentType describes the payload format of a pipe frame.
type ContentType string

const (
	ContentText   ContentType = "text"
	ContentJSON   ContentType = "json"
	ContentBinary ContentType = "binary"
	ContentEOS    ContentType = "eos"
)

// Frame is a single message in the pipe stream.
type Frame struct {
	Seq     uint64      `json:"seq"`
	Type    ContentType `json:"type"`
	Payload string      `json:"payload,omitempty"`
}

// Encoder writes pipe frames to an io.Writer.
type Encoder struct {
	w   io.Writer
	seq uint64
}

// NewEncoder creates a new frame encoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, seq: 0}
}

// Encode writes a frame to the stream.
func (e *Encoder) Encode(contentType ContentType, payload string) error {
	e.seq++
	f := Frame{
		Seq:     e.seq,
		Type:    contentType,
		Payload: payload,
	}
	return json.NewEncoder(e.w).Encode(f)
}

// EncodeEOS sends an end-of-stream signal.
func (e *Encoder) EncodeEOS() error {
	return e.Encode(ContentEOS, "")
}

// Decoder reads pipe frames from an io.Reader.
type Decoder struct {
	r     *bufio.Scanner
	seq   uint64
	done  bool
}

// NewDecoder creates a new frame decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:   bufio.NewScanner(r),
		seq: 0,
	}
}

// Decode reads the next frame from the stream.
// Returns io.EOF when the stream ends normally.
func (d *Decoder) Decode() (*Frame, error) {
	if d.done {
		return nil, io.EOF
	}

	if !d.r.Scan() {
		d.done = true
		if err := d.r.Err(); err != nil {
			return nil, fmt.Errorf("pipe: read error: %w", err)
		}
		return nil, io.EOF
	}

	line := d.r.Text()
	if strings.TrimSpace(line) == "" {
		return d.Decode() // Skip empty lines
	}

	var f Frame
	if err := json.Unmarshal([]byte(line), &f); err != nil {
		return nil, fmt.Errorf("pipe: invalid frame: %w", err)
	}

	if f.Type == ContentEOS {
		d.done = true
	}

	return &f, nil
}

// Pipe connects the output of one command to the input of another.
// It reads frames from `input`, and for each non-EOS frame, passes
// the payload to `processFn`. The function should handle each payload
// and optionally produce output.
type Pipe struct {
	Input io.Reader
}

// Run reads all frames, collecting payloads, and returns them.
// Returns when EOS is received or the stream ends.
func (p *Pipe) Run() ([]string, error) {
	dec := NewDecoder(p.Input)
	var results []string

	for {
		frame, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch frame.Type {
		case ContentText, ContentJSON:
			results = append(results, frame.Payload)
		case ContentBinary:
			decoded, err := base64.StdEncoding.DecodeString(frame.Payload)
			if err != nil {
				results = append(results, frame.Payload) // fallback
			} else {
				results = append(results, string(decoded))
			}
		case ContentEOS:
			return results, nil
		}
	}

	return results, nil
}
