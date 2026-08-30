package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var checkpointRE = regexp.MustCompile(`^v0\.(\d+)$`)

type authority struct {
	path    string
	current *regexp.Regexp
}

var authorities = []authority{
	{"docs/status/versioning-and-release-status.md", regexp.MustCompile(`current milestone position is \*\*(v0\.\d+)\*\*`)},
	{"docs/status/versioning-and-release-status.ja.md", regexp.MustCompile(`現在のmilestone位置は\s*\*\*(v0\.\d+)\*\*`)},
	{"docs/IMPLEMENTATION_STATUS.md", regexp.MustCompile(`current milestone position is \*\*(v0\.\d+)\*\*`)},
	{"docs/IMPLEMENTATION_STATUS.ja.md", regexp.MustCompile(`現在のmilestone位置は\s*\*\*(v0\.\d+)\*\*`)},
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, `usage: tools/bump-milestone v0.N "Gate Name"`)
		os.Exit(2)
	}
	root, err := findRoot()
	if err != nil {
		fail(err)
	}
	if err := bump(root, os.Args[1], os.Args[2]); err != nil {
		fail(err)
	}
	fmt.Printf("checkpoint advanced to %s\n", os.Args[1])

	cmd := exec.Command("python3", "tools/check_docs.py")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("documentation check failed after checkpoint update: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "bump-milestone:", err)
	os.Exit(1)
}

func findRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "docs", "status", "versioning-and-release-status.md")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("repository root not found")
		}
	}
}

func bump(root, next, gate string) error {
	gate = strings.TrimSpace(gate)
	if gate == "" || strings.ContainsAny(gate, "\r\n|") {
		return errors.New("gate name must be non-empty and must not contain newlines or '|'")
	}
	nextMinor, err := parseCheckpoint(next)
	if err != nil {
		return err
	}

	contents := map[string][]byte{}
	current := ""
	for _, a := range authorities {
		data, err := os.ReadFile(filepath.Join(root, a.path))
		if err != nil {
			return fmt.Errorf("read %s: %w", a.path, err)
		}
		matches := a.current.FindAllSubmatch(data, -1)
		if len(matches) != 1 {
			return fmt.Errorf("%s: expected exactly one current checkpoint declaration, found %d", a.path, len(matches))
		}
		value := string(matches[0][1])
		if current == "" {
			current = value
		} else if value != current {
			return fmt.Errorf("checkpoint authorities disagree: %s vs %s in %s", current, value, a.path)
		}
		contents[a.path] = data
	}
	currentMinor, err := parseCheckpoint(current)
	if err != nil {
		return fmt.Errorf("invalid current checkpoint %q: %w", current, err)
	}
	if nextMinor != currentMinor+1 {
		return fmt.Errorf("next checkpoint must be v0.%d; got %s", currentMinor+1, next)
	}

	for _, a := range authorities {
		data := contents[a.path]
		data = a.current.ReplaceAllFunc(data, func(match []byte) []byte {
			return bytes.Replace(match, []byte(current), []byte(next), 1)
		})
		contents[a.path] = data
	}

	for _, path := range []string{"docs/status/versioning-and-release-status.md", "docs/status/versioning-and-release-status.ja.md"} {
		data := contents[path]
		rowRE := regexp.MustCompile(`(?m)^\|\s*` + regexp.QuoteMeta(current) + `\s*\|[^\n]*$`)
		loc := rowRE.FindIndex(data)
		if loc == nil {
			return fmt.Errorf("%s: current checkpoint row %s not found", path, current)
		}
		state := "✅ implemented"
		if strings.HasSuffix(path, ".ja.md") {
			state = "実装済み"
		}
		row := fmt.Sprintf("\n| %s | %s | %s |", next, gate, state)
		data = append(data[:loc[1]], append([]byte(row), data[loc[1]:]...)...)
		contents[path] = data
	}

	generatedPath := "internal/buildinfo/checkpoint_generated.go"
	generated, err := os.ReadFile(filepath.Join(root, generatedPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", generatedPath, err)
	}
	generatedRE := regexp.MustCompile(`const GeneratedCheckpoint = "v0\.\d+"`)
	if len(generatedRE.FindAll(generated, -1)) != 1 {
		return fmt.Errorf("%s: expected exactly one generated checkpoint constant", generatedPath)
	}
	contents[generatedPath] = generatedRE.ReplaceAll(generated, []byte(`const GeneratedCheckpoint = "`+next+`"`))

	for path, data := range contents {
		if err := writeAtomic(filepath.Join(root, path), data); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func parseCheckpoint(value string) (int, error) {
	match := checkpointRE.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 2 {
		return 0, fmt.Errorf("checkpoint must match v0.N: %q", value)
	}
	minor, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	return minor, nil
}

func writeAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bump-milestone-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
