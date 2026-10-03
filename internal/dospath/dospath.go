// Package dospath turns the DOS-style paths stored in CONFIG.RA and other
// RA/EleBBS data files into paths the host OS can open. On Windows every
// function returns its input unchanged.
package dospath

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ToHost rewrites a DOS path for the host: `\` becomes `/` and a drive
// letter is removed, so `C:\ELE\TXTFILES` becomes `/ELE/TXTFILES`. It only
// rewrites the string; Resolve also matches names ignoring case.
func ToHost(p string) string {
	if runtime.GOOS == "windows" {
		return p
	}
	if len(p) >= 2 && p[1] == ':' && isLetter(p[0]) {
		p = p[2:]
	}
	return strings.ReplaceAll(p, `\`, "/")
}

// Resolve returns the host path of an existing file or directory named by
// p, matching each path element with ASCII letters folded, as DOS does.
// An element whose exact name exists is always preferred. From the first
// element that matches nothing, the rest of ToHost(p) is kept as written, so
// a file about to be created lands in the existing directory and an open of
// a missing file still fails.
func Resolve(p string) string {
	if runtime.GOOS == "windows" {
		return p
	}
	h := filepath.Clean(ToHost(p))
	if _, err := os.Lstat(h); err == nil {
		return h
	}
	cur, rest := "", h
	if filepath.IsAbs(h) {
		cur, rest = "/", h[1:]
	}
	parts := strings.Split(rest, "/")
	unmatched := func(i int) string {
		return filepath.Join(append([]string{cur}, parts[i:]...)...)
	}
	for i, part := range parts {
		next := filepath.Join(cur, part)
		if _, err := os.Lstat(next); err == nil {
			cur = next
			continue
		}
		dir := cur
		if dir == "" {
			dir = "."
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return unmatched(i)
		}
		found := ""
		for _, e := range entries {
			if equalFoldASCII(e.Name(), part) {
				found = e.Name()
				break
			}
		}
		if found == "" {
			return unmatched(i)
		}
		cur = filepath.Join(cur, found)
	}
	return cur
}

// equalFoldASCII compares byte by byte, folding only a-z. strings.EqualFold
// decodes UTF-8, so every invalid CP437 byte would match every other.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func isLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
