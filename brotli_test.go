// Copyright 2016 Google Inc. All Rights Reserved.
//
// Distributed under MIT license.
// See file LICENSE for detail or copy at https://opensource.org/licenses/MIT

package brotli

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"
)

func checkCompressedData(compressedData, wantOriginalData []byte) error {
	uncompressed, err := Decode(compressedData)
	if err != nil {
		return fmt.Errorf("brotli decompress failed: %v", err)
	}
	if !bytes.Equal(uncompressed, wantOriginalData) {
		if len(wantOriginalData) != len(uncompressed) {
			return fmt.Errorf(""+
				"Data doesn't uncompress to the original value.\n"+
				"Length of original: %v\n"+
				"Length of uncompressed: %v",
				len(wantOriginalData), len(uncompressed))
		}
		for i := range wantOriginalData {
			if wantOriginalData[i] != uncompressed[i] {
				return fmt.Errorf(""+
					"Data doesn't uncompress to the original value.\n"+
					"Original at %v is %v\n"+
					"Uncompressed at %v is %v",
					i, wantOriginalData[i], i, uncompressed[i])
			}
		}
	}
	return nil
}

func TestEncoderNoWrite(t *testing.T) {
	out := bytes.Buffer{}
	e := NewWriterOptions(&out, WriterOptions{Quality: 5})
	if err := e.Close(); err != nil {
		t.Errorf("Close()=%v, want nil", err)
	}
	// Check Write after close.
	if _, err := e.Write([]byte("hi")); err == nil {
		t.Errorf("No error after Close() + Write()")
	}
}

func TestEncoderEmptyWrite(t *testing.T) {
	out := bytes.Buffer{}
	e := NewWriterOptions(&out, WriterOptions{Quality: 5})
	n, err := e.Write([]byte(""))
	if n != 0 || err != nil {
		t.Errorf("Write()=%v,%v, want 0, nil", n, err)
	}
	if err := e.Close(); err != nil {
		t.Errorf("Close()=%v, want nil", err)
	}
}

func TestWriter(t *testing.T) {
	for level := BestSpeed; level <= BestCompression; level++ {
		// Test basic encoder usage.
		input := []byte("<html><body><H1>Hello world</H1></body></html>")
		out := bytes.Buffer{}
		e := NewWriterOptions(&out, WriterOptions{Quality: level})
		in := bytes.NewReader([]byte(input))
		n, err := io.Copy(e, in)
		if err != nil {
			t.Errorf("Copy Error: %v", err)
		}
		if int(n) != len(input) {
			t.Errorf("Copy() n=%v, want %v", n, len(input))
		}
		if err := e.Close(); err != nil {
			t.Errorf("Close Error after copied %d bytes: %v", n, err)
		}
		if err := checkCompressedData(out.Bytes(), input); err != nil {
			t.Error(err)
		}

		out2 := bytes.Buffer{}
		e.Reset(&out2)
		n2, err := e.Write(input)
		if err != nil {
			t.Errorf("Write error after Reset: %v", err)
		}
		if n2 != len(input) {
			t.Errorf("Write() after Reset n=%d, want %d", n2, len(input))
		}
		if err := e.Close(); err != nil {
			t.Errorf("Close error after Reset (copied %d) bytes: %v", n2, err)
		}
		if !bytes.Equal(out.Bytes(), out2.Bytes()) {
			t.Error("Compressed data after Reset doesn't equal first time")
		}
	}
}

func TestIssue22(t *testing.T) {
	f, err := os.Open("testdata/issue22.gz")
	if err != nil {
		t.Fatalf("Error opening test data file: %v", err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Error creating gzip reader: %v", err)
	}

	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("Error reading test data: %v", err)
	}

	if len(data) != 2851073 {
		t.Fatalf("Wrong length for test data: got %d, want 2851073", len(data))
	}

	for level := BestSpeed; level <= BestCompression; level++ {
		out := bytes.Buffer{}
		e := NewWriterOptions(&out, WriterOptions{Quality: level})
		n, err := e.Write(data)
		if err != nil {
			t.Errorf("Error compressing data: %v", err)
		}
		if int(n) != len(data) {
			t.Errorf("Write() n=%v, want %v", n, len(data))
		}
		if err := e.Close(); err != nil {
			t.Errorf("Close Error after writing %d bytes: %v", n, err)
		}
		if err := checkCompressedData(out.Bytes(), data); err != nil {
			t.Errorf("Error decompressing data at level %d: %v", level, err)
		}
	}
}

func TestEncoderStreams(t *testing.T) {
	// Test that output is streamed.
	// Adjust window size to ensure the encoder outputs at least enough bytes
	// to fill the window.
	const lgWin = 16
	windowSize := int(math.Pow(2, lgWin))
	input := make([]byte, 8*windowSize)
	rand.Read(input)
	out := bytes.Buffer{}
	e := NewWriterOptions(&out, WriterOptions{Quality: 11, LGWin: lgWin})
	halfInput := input[:len(input)/2]
	in := bytes.NewReader(halfInput)

	n, err := io.Copy(e, in)
	if err != nil {
		t.Errorf("Copy Error: %v", err)
	}

	// We've fed more data than the sliding window size. Check that some
	// compressed data has been output.
	if out.Len() == 0 {
		t.Errorf("Output length is 0 after %d bytes written", n)
	}
	if err := e.Close(); err != nil {
		t.Errorf("Close Error after copied %d bytes: %v", n, err)
	}
	if err := checkCompressedData(out.Bytes(), halfInput); err != nil {
		t.Error(err)
	}
}

func TestEncoderLargeInput(t *testing.T) {
	for level := BestSpeed; level <= BestCompression; level++ {
		input := make([]byte, 1000000)
		rand.Read(input)
		out := bytes.Buffer{}
		e := NewWriterOptions(&out, WriterOptions{Quality: level})
		in := bytes.NewReader(input)

		n, err := io.Copy(e, in)
		if err != nil {
			t.Errorf("Copy Error: %v", err)
		}
		if int(n) != len(input) {
			t.Errorf("Copy() n=%v, want %v", n, len(input))
		}
		if err := e.Close(); err != nil {
			t.Errorf("Close Error after copied %d bytes: %v", n, err)
		}
		if err := checkCompressedData(out.Bytes(), input); err != nil {
			t.Error(err)
		}

		out2 := bytes.Buffer{}
		e.Reset(&out2)
		n2, err := e.Write(input)
		if err != nil {
			t.Errorf("Write error after Reset: %v", err)
		}
		if n2 != len(input) {
			t.Errorf("Write() after Reset n=%d, want %d", n2, len(input))
		}
		if err := e.Close(); err != nil {
			t.Errorf("Close error after Reset (copied %d) bytes: %v", n2, err)
		}
		if !bytes.Equal(out.Bytes(), out2.Bytes()) {
			t.Error("Compressed data after Reset doesn't equal first time")
		}
	}
}

func TestEncoderFlush(t *testing.T) {
	input := make([]byte, 1000)
	rand.Read(input)
	out := bytes.Buffer{}
	e := NewWriterOptions(&out, WriterOptions{Quality: 5})
	in := bytes.NewReader(input)
	_, err := io.Copy(e, in)
	if err != nil {
		t.Fatalf("Copy Error: %v", err)
	}
	if err := e.Flush(); err != nil {
		t.Fatalf("Flush(): %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("0 bytes written after Flush()")
	}
	decompressed := make([]byte, 1000)
	reader := NewReader(bytes.NewReader(out.Bytes()))
	n, err := reader.Read(decompressed)
	if n != len(decompressed) || err != nil {
		t.Errorf("Expected <%v, nil>, but <%v, %v>", len(decompressed), n, err)
	}
	if !bytes.Equal(decompressed, input) {
		t.Errorf(""+
			"Decompress after flush: %v\n"+
			"%q\n"+
			"want:\n%q",
			err, decompressed, input)
	}
	if err := e.Close(); err != nil {
		t.Errorf("Close(): %v", err)
	}
}

type readerWithTimeout struct {
	io.Reader
}

func (r readerWithTimeout) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result)
	go func() {
		n, err := r.Reader.Read(p)
		ch <- result{n, err}
	}()
	select {
	case result := <-ch:
		return result.n, result.err
	case <-time.After(5 * time.Second):
		return 0, fmt.Errorf("read timed out")
	}
}

func TestDecoderStreaming(t *testing.T) {
	pr, pw := io.Pipe()
	writer := NewWriterOptions(pw, WriterOptions{Quality: 5, LGWin: 20})
	reader := readerWithTimeout{NewReader(pr)}
	defer func() {
		go ioutil.ReadAll(pr) // swallow the "EOF" token from writer.Close
		if err := writer.Close(); err != nil {
			t.Errorf("writer.Close: %v", err)
		}
	}()

	ch := make(chan []byte)
	errch := make(chan error)
	go func() {
		for {
			segment, ok := <-ch
			if !ok {
				return
			}
			if n, err := writer.Write(segment); err != nil || n != len(segment) {
				errch <- fmt.Errorf("write=%v,%v, want %v,%v", n, err, len(segment), nil)
				return
			}
			if err := writer.Flush(); err != nil {
				errch <- fmt.Errorf("flush: %v", err)
				return
			}
		}
	}()
	defer close(ch)

	segments := [...][]byte{
		[]byte("first"),
		[]byte("second"),
		[]byte("third"),
	}
	for k, segment := range segments {
		t.Run(fmt.Sprintf("Segment%d", k), func(t *testing.T) {
			select {
			case ch <- segment:
			case err := <-errch:
				t.Fatalf("write: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out")
			}
			wantLen := len(segment)
			got := make([]byte, wantLen)
			if n, err := reader.Read(got); err != nil || n != wantLen || !bytes.Equal(got, segment) {
				t.Fatalf("read[%d]=%q,%v,%v, want %q,%v,%v", k, got, n, err, segment, wantLen, nil)
			}
		})
	}
}

func TestReader(t *testing.T) {
	content := bytes.Repeat([]byte("hello world!"), 10000)
	encoded, _ := Encode(content, WriterOptions{Quality: 5})
	r := NewReader(bytes.NewReader(encoded))
	var decodedOutput bytes.Buffer
	n, err := io.Copy(&decodedOutput, r)
	if err != nil {
		t.Fatalf("Copy(): n=%v, err=%v", n, err)
	}
	if got := decodedOutput.Bytes(); !bytes.Equal(got, content) {
		t.Errorf(""+
			"Reader output:\n"+
			"%q\n"+
			"want:\n"+
			"<%d bytes>",
			got, len(content))
	}

	r.Reset(bytes.NewReader(encoded))
	decodedOutput.Reset()
	n, err = io.Copy(&decodedOutput, r)
	if err != nil {
		t.Fatalf("After Reset: Copy(): n=%v, err=%v", n, err)
	}
	if got := decodedOutput.Bytes(); !bytes.Equal(got, content) {
		t.Errorf("After Reset: "+
			"Reader output:\n"+
			"%q\n"+
			"want:\n"+
			"<%d bytes>",
			got, len(content))
	}

}

func TestDecode(t *testing.T) {
	content := bytes.Repeat([]byte("hello world!"), 10000)
	encoded, _ := Encode(content, WriterOptions{Quality: 5})
	decoded, err := Decode(encoded)
	if err != nil {
		t.Errorf("Decode: %v", err)
	}
	if !bytes.Equal(decoded, content) {
		t.Errorf(""+
			"Decode content:\n"+
			"%q\n"+
			"want:\n"+
			"<%d bytes>",
			decoded, len(content))
	}
}

func TestQuality(t *testing.T) {
	content := bytes.Repeat([]byte("hello world!"), 10000)
	for q := 0; q < 12; q++ {
		encoded, _ := Encode(content, WriterOptions{Quality: q})
		decoded, err := Decode(encoded)
		if err != nil {
			t.Errorf("Decode: %v", err)
		}
		if !bytes.Equal(decoded, content) {
			t.Errorf(""+
				"Decode content:\n"+
				"%q\n"+
				"want:\n"+
				"<%d bytes>",
				decoded, len(content))
		}
	}
}

func TestDecodeFuzz(t *testing.T) {
	// Test that the decoder terminates with corrupted input.
	content := bytes.Repeat([]byte("hello world!"), 100)
	rnd := rand.New(rand.NewSource(0))
	encoded, err := Encode(content, WriterOptions{Quality: 5})
	if err != nil {
		t.Fatalf("Encode(<%d bytes>, _) = _, %s", len(content), err)
	}
	if len(encoded) == 0 {
		t.Fatalf("Encode(<%d bytes>, _) produced empty output", len(content))
	}
	for i := 0; i < 100; i++ {
		enc := append([]byte{}, encoded...)
		for j := 0; j < 5; j++ {
			enc[rnd.Intn(len(enc))] = byte(rnd.Intn(256))
		}
		Decode(enc)
	}
}

func TestDecodeTrailingData(t *testing.T) {
	content := bytes.Repeat([]byte("hello world!"), 100)
	encoded, _ := Encode(content, WriterOptions{Quality: 5})
	_, err := Decode(append(encoded, 0))
	if err == nil {
		t.Errorf("Expected 'excessive input' error")
	}
}

func TestEncodeDecode(t *testing.T) {
	for _, test := range []struct {
		data    []byte
		repeats int
	}{
		{nil, 0},
		{[]byte("A"), 1},
		{[]byte("<html><body><H1>Hello world</H1></body></html>"), 10},
		{[]byte("<html><body><H1>Hello world</H1></body></html>"), 1000},
	} {
		t.Logf("case %q x %d", test.data, test.repeats)
		input := bytes.Repeat(test.data, test.repeats)
		encoded, err := Encode(input, WriterOptions{Quality: 5})
		if err != nil {
			t.Errorf("Encode: %v", err)
		}
		// Inputs are compressible, but may be too small to compress.
		if maxSize := len(input)/2 + 20; len(encoded) >= maxSize {
			t.Errorf(""+
				"Encode returned %d bytes, want <%d\n"+
				"Encoded=%q",
				len(encoded), maxSize, encoded)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Errorf("Decode: %v", err)
		}
		if !bytes.Equal(decoded, input) {
			var want string
			if len(input) > 320 {
				want = fmt.Sprintf("<%d bytes>", len(input))
			} else {
				want = fmt.Sprintf("%q", input)
			}
			t.Errorf(""+
				"Decode content:\n"+
				"%q\n"+
				"want:\n"+
				"%s",
				decoded, want)
		}
	}
}

// Encode returns content encoded with Brotli.
func Encode(content []byte, options WriterOptions) ([]byte, error) {
	var buf bytes.Buffer
	writer := NewWriterOptions(&buf, options)
	_, err := writer.Write(content)
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return buf.Bytes(), err
}

// Decode decodes Brotli encoded data.
func Decode(encodedData []byte) ([]byte, error) {
	r := NewReader(bytes.NewReader(encodedData))
	return ioutil.ReadAll(r)
}

func BenchmarkEncodeLevels(b *testing.B) {
	opticks, err := ioutil.ReadFile("testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := BestSpeed; level <= BestCompression; level++ {
		b.Run(fmt.Sprintf("%d", level), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(opticks)))
			for i := 0; i < b.N; i++ {
				w := NewWriterLevel(ioutil.Discard, level)
				w.Write(opticks)
				w.Close()
			}
		})
	}
}

func BenchmarkEncodeLevelsReset(b *testing.B) {
	opticks, err := ioutil.ReadFile("testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := BestSpeed; level <= BestCompression; level++ {
		buf := new(bytes.Buffer)
		w := NewWriterLevel(buf, level)
		w.Write(opticks)
		w.Close()
		b.Run(fmt.Sprintf("%d", level), func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(len(opticks))/float64(buf.Len()), "ratio")
			b.SetBytes(int64(len(opticks)))
			for i := 0; i < b.N; i++ {
				w.Reset(ioutil.Discard)
				w.Write(opticks)
				w.Close()
			}
		})
	}
}

func BenchmarkDecodeLevels(b *testing.B) {
	opticks, err := ioutil.ReadFile("testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := BestSpeed; level <= BestCompression; level++ {
		buf := new(bytes.Buffer)
		w := NewWriterLevel(buf, level)
		w.Write(opticks)
		w.Close()
		compressed := buf.Bytes()
		b.Run(fmt.Sprintf("%d", level), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(opticks)))
			for i := 0; i < b.N; i++ {
				io.Copy(ioutil.Discard, NewReader(bytes.NewReader(compressed)))
			}
		})
	}
}
