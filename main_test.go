package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

type countWriter struct {
	err error
	n   int
}

func (w *countWriter) Write([]byte) (int, error) { w.n++; return 0, w.err }

// failFirst fails on the first write, then succeeds.
type failFirst struct {
	n   int
	buf bytes.Buffer
}

func (w *failFirst) Write(p []byte) (int, error) {
	w.n++
	if w.n == 1 {
		return 0, errors.New("boom")
	}
	return w.buf.Write(p)
}

func console(buf *bytes.Buffer) io.Writer {
	return zerolog.ConsoleWriter{Out: buf, NoColor: true}
}

func TestProcessInput(t *testing.T) {
	line := `{"level":"info","message":"hello","key":"val"}`

	t.Run("valid line", func(t *testing.T) {
		var buf bytes.Buffer
		if err := processInput(strings.NewReader(line+"\n"), console(&buf)); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"INF", "hello", "key=val"} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("output %q missing %q", buf.String(), want)
			}
		}
	})

	t.Run("multiple lines in order", func(t *testing.T) {
		var buf bytes.Buffer
		in := `{"level":"info","message":"first"}` + "\n" + `{"level":"warn","message":"second"}` + "\n"
		if err := processInput(strings.NewReader(in), console(&buf)); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 || !strings.Contains(lines[0], "first") || !strings.Contains(lines[1], "second") {
			t.Errorf("unexpected output: %q", buf.String())
		}
	})

	t.Run("empty input", func(t *testing.T) {
		var buf bytes.Buffer
		if err := processInput(strings.NewReader(""), console(&buf)); err != nil || buf.Len() != 0 {
			t.Errorf("err=%v out=%q", err, buf.String())
		}
	})

	t.Run("non-JSON line does not stop loop", func(t *testing.T) {
		var buf bytes.Buffer
		in := "not json\n" + line + "\n"
		if err := processInput(strings.NewReader(in), console(&buf)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "hello") {
			t.Errorf("second line not processed: %q", buf.String())
		}
	})

	t.Run("io.EOF from writer breaks loop", func(t *testing.T) {
		w := &countWriter{err: io.EOF}
		if err := processInput(strings.NewReader("a\nb\n"), w); err != nil || w.n != 1 {
			t.Errorf("err=%v writes=%d, want nil and 1", err, w.n)
		}
	})

	t.Run("other writer error falls back to stdout and continues", func(t *testing.T) {
		r, w, _ := os.Pipe()
		orig := os.Stdout
		os.Stdout = w
		fw := &failFirst{}
		err := processInput(strings.NewReader("raw1\nraw2\n"), fw)
		os.Stdout = orig
		_ = w.Close()
		out, _ := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "raw1\n" {
			t.Errorf("stdout = %q", out)
		}
		if fw.buf.String() != "raw2" {
			t.Errorf("second line not written: %q", fw.buf.String())
		}
	})

	t.Run("too long line", func(t *testing.T) {
		in := strings.Repeat("x", bufio.MaxScanTokenSize+1)
		err := processInput(strings.NewReader(in), console(&bytes.Buffer{}))
		if !errors.Is(err, bufio.ErrTooLong) {
			t.Errorf("got %v", err)
		}
	})
}

// TestHelperProcess re-execs the test binary as the real main().
func TestHelperProcess(t *testing.T) {
	if os.Getenv("PZ_TEST_MAIN") != "1" {
		t.Skip("helper process")
	}
	os.Args = append([]string{"pretty-zerolog"}, strings.Fields(os.Getenv("PZ_ARGS"))...)
	main()
	os.Exit(0)
}

func runMain(t *testing.T, stdin io.Reader, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "PZ_TEST_MAIN=1", "PZ_ARGS="+strings.Join(args, " "))
	if stdin == nil {
		f, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		cmd.Stdin = f
	} else {
		cmd.Stdin = stdin
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), code
}

func TestMain_(t *testing.T) {
	const logLine = `{"level":"info","message":"hello","key":"val"}` + "\n"

	for _, flag := range []string{"-v", "--version"} {
		out, code := runMain(t, strings.NewReader("{}\n"), flag)
		if code != 0 || !strings.Contains(out, "pretty-zerolog dev (commit none, built unknown)") {
			t.Errorf("%s: code=%d out=%q", flag, code, out)
		}
	}

	t.Run("stdin pipe", func(t *testing.T) {
		out, code := runMain(t, strings.NewReader(logLine))
		if code != 0 || !strings.Contains(out, "hello") {
			t.Errorf("code=%d out=%q", code, out)
		}
	})

	for _, tf := range []string{"full", "default"} {
		t.Run("time-format "+tf, func(t *testing.T) {
			out, code := runMain(t, strings.NewReader(logLine), "--time-format", tf)
			if code != 0 || !strings.Contains(out, "hello") {
				t.Errorf("code=%d out=%q", code, out)
			}
		})
	}

	t.Run("bad time-format", func(t *testing.T) {
		if _, code := runMain(t, strings.NewReader(logLine), "--time-format", "bogus"); code == 0 {
			t.Error("expected non-zero exit")
		}
	})

	t.Run("files in order", func(t *testing.T) {
		dir := t.TempDir()
		a, b := filepath.Join(dir, "a.jsonl"), filepath.Join(dir, "b.jsonl")
		_ = os.WriteFile(a, []byte(`{"level":"info","message":"from-a"}`+"\n"), 0o600)
		_ = os.WriteFile(b, []byte(`{"level":"info","message":"from-b"}`+"\n"), 0o600)
		out, code := runMain(t, nil, a, b)
		ia, ib := strings.Index(out, "from-a"), strings.Index(out, "from-b")
		if code != 0 || ia < 0 || ib < ia {
			t.Errorf("code=%d out=%q", code, out)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		out, code := runMain(t, nil, filepath.Join(t.TempDir(), "nope"))
		if code != 1 || !strings.Contains(out, "open:") {
			t.Errorf("code=%d out=%q", code, out)
		}
	})

	t.Run("usage", func(t *testing.T) {
		out, code := runMain(t, nil)
		if code != 1 || !strings.Contains(out, "Usage:") {
			t.Errorf("code=%d out=%q", code, out)
		}
	})
}
