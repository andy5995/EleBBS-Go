package dospath

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestToHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`c:\ele\txtfiles\`, "/ele/txtfiles/"},
		{`C:\ELE\MSGBASE`, "/ELE/MSGBASE"},
		{`\ele\msgbase`, "/ele/msgbase"},
		{`txtfiles\welcome\WELCOME.ANS`, "txtfiles/welcome/WELCOME.ANS"},
		{`d:files`, "files"},
		{`/tmp/bbs\users.bbs`, "/tmp/bbs/users.bbs"},
		{"/tmp/bbs/users.bbs", "/tmp/bbs/users.bbs"},
		{"", ""},
	}
	for _, c := range cases {
		want := c.want
		if runtime.GOOS == "windows" {
			want = c.in
		}
		if got := ToHost(c.in); got != want {
			t.Errorf("ToHost(%q) = %q, want %q", c.in, got, want)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func readResolved(t *testing.T, spec string) string {
	t.Helper()
	b, err := os.ReadFile(Resolve(spec))
	if err != nil {
		t.Fatalf("Resolve(%q): %v", spec, err)
	}
	return string(b)
}

func TestResolveExactPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "WELCOME.ANS")
	writeFile(t, p, "hi")
	if got := Resolve(p); got != p {
		t.Fatalf("Resolve(%q) = %q", p, got)
	}
}

func TestResolveFileNameCase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "WELCOME.ANS"), "hi")
	if got := readResolved(t, filepath.Join(dir, "welcome.ans")); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveDirectoryCase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "txtfiles", "Welcome", "WELCOME.ANS"), "hi")
	if got := readResolved(t, filepath.Join(dir, "TXTFILES", "WELCOME", "welcome.ans")); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveDOSSeparators(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "txtfiles", "WELCOME.ANS"), "hi")
	if got := readResolved(t, dir+`\TXTFILES\welcome.ans`); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "MsgBase"), 0755); err != nil {
		t.Fatal(err)
	}
	got := Resolve(filepath.Join(dir, "MSGBASE") + `\`)
	if st, err := os.Stat(got); err != nil || !st.IsDir() {
		t.Fatalf("Resolve gave %q, not the MsgBase directory", got)
	}
}

func TestResolveMissingReturnsHostPath(t *testing.T) {
	dir := t.TempDir()
	spec := dir + `\NOPE\MISSING.TXT`
	if got, want := Resolve(spec), ToHost(spec); got != filepath.Clean(want) {
		t.Fatalf("Resolve(%q) = %q, want %q", spec, got, filepath.Clean(want))
	}
}

// Only a case-sensitive filesystem can hold both names.
func TestResolvePrefersExactCase(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("filesystem is case-insensitive")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.ans"), "lower")
	writeFile(t, filepath.Join(dir, "A.ANS"), "upper")
	if got := readResolved(t, filepath.Join(dir, "A.ANS")); got != "upper" {
		t.Fatalf("got %q", got)
	}
}

// A file about to be created goes in the existing directory, whatever case
// the caller used for it.
func TestResolveNewFileInExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "MsgBase"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, Resolve(filepath.Join(dir, "MSGBASE", "USERSIDX.BBS")), "idx")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want only MsgBase in %s, got %d entries", dir, len(entries))
	}
	if _, err := os.Stat(filepath.Join(dir, "MsgBase", "USERSIDX.BBS")); err != nil {
		t.Fatal(err)
	}
}

// RA paths are CP437 bytes, often not valid UTF-8. Only ASCII letters fold;
// \x81 (ü) and \x82 (é) are different names.
func TestResolveFoldsOnlyASCII(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold raw CP437 bytes")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "\x82.TXT"), "e-acute")
	if _, err := os.ReadFile(Resolve(filepath.Join(dir, "\x81.txt"))); err == nil {
		t.Fatal("\\x81.txt resolved to \\x82.TXT")
	}
}
