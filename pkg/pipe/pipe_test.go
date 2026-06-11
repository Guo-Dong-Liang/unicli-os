package pipe

import (
	"strings"
	"testing"
)

func TestParsePipeline_TwoStages(t *testing.T) {
	p := ParsePipeline("hello.say --name 果果 | image.resize --width 800")
	if len(p.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(p.Stages))
	}
	if p.Stages[0].Args[0] != "hello.say" {
		t.Errorf("expected stage 0 tool 'hello.say', got %q", p.Stages[0].Args[0])
	}
	if p.Stages[1].Args[0] != "image.resize" {
		t.Errorf("expected stage 1 tool 'image.resize', got %q", p.Stages[1].Args[0])
	}
}

func TestParsePipeline_ThreeStages(t *testing.T) {
	p := ParsePipeline("a | b | c")
	if len(p.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(p.Stages))
	}
}

func TestParsePipeline_NoPipe(t *testing.T) {
	p := ParsePipeline("just.a.tool")
	if len(p.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(p.Stages))
	}
}

func TestParsePipeline_Empty(t *testing.T) {
	p := ParsePipeline("")
	if len(p.Stages) != 0 {
		t.Fatalf("expected 0 stages, got %d", len(p.Stages))
	}
}

func TestEncoderDecoder_RoundTrip(t *testing.T) {
	var buf strings.Builder
	enc := NewEncoder(&buf)

	if err := enc.Encode(ContentText, "hello world"); err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if err := enc.Encode(ContentJSON, `{"key":"val"}`); err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if err := enc.EncodeEOS(); err != nil {
		t.Fatalf("encode EOS failed: %v", err)
	}

	dec := NewDecoder(strings.NewReader(buf.String()))

	f1, err := dec.Decode()
	if err != nil {
		t.Fatalf("decode frame1 failed: %v", err)
	}
	if f1.Type != ContentText || f1.Payload != "hello world" {
		t.Errorf("frame1: expected text/hello world, got %s/%s", f1.Type, f1.Payload)
	}
	if f1.Seq != 1 {
		t.Errorf("frame1 seq: expected 1, got %d", f1.Seq)
	}

	f2, _ := dec.Decode()
	if f2.Type != ContentJSON || f2.Payload != `{"key":"val"}` {
		t.Errorf("frame2: expected json/{\"key\":\"val\"}, got %s/%s", f2.Type, f2.Payload)
	}

	f3, err := dec.Decode()
	if err != nil {
		t.Fatalf("decode frame3 failed: %v", err)
	}
	if f3.Type != ContentEOS {
		t.Errorf("frame3: expected EOS, got %s", f3.Type)
	}

	// After EOS, should get EOF
	_, err = dec.Decode()
	if err == nil {
		t.Error("expected EOF after EOS")
	}
}

func TestPipeRun_CollectsText(t *testing.T) {
	input := strings.NewReader(`{"seq":1,"type":"text","payload":"line1"}
{"seq":2,"type":"text","payload":"line2"}
{"seq":3,"type":"eos"}
`)
	p := &Pipe{Input: input}
	results, err := p.Run()
	if err != nil {
		t.Fatalf("Pipe.Run failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != "line1" || results[1] != "line2" {
		t.Errorf("results mismatch: %v", results)
	}
}
