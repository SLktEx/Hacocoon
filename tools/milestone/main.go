package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const checkpointSourcePath = "docs/status/checkpoints.yaml"

var checkpointRE = regexp.MustCompile(`^v0\.(\d+)$`)
var milestoneLineRE = regexp.MustCompile(`^  "(v0\.\d+)": ("(?:[^"\\]|\\.)*")$`)
var currentLineRE = regexp.MustCompile(`^current: ("v0\.\d+")$`)

type authority struct {
	path    string
	current *regexp.Regexp
}

type milestone struct {
	Version string
	Gate    string
	Minor   int
}

type checkpointSource struct {
	Schema     int
	Current    string
	Milestones []milestone
}

type fileUpdate struct {
	Path     string
	Original []byte
	Updated  []byte
	Mode     os.FileMode
}

type writeFileFunc func(path string, data []byte, mode os.FileMode) error

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
	if err := bumpWithValidation(root, os.Args[1], os.Args[2], func() error {
		return runDocsCheck(root)
	}); err != nil {
		fail(err)
	}
	fmt.Printf("checkpoint advanced to %s\n", os.Args[1])
}

func runDocsCheck(root string) error {
	cmd := exec.Command("python3", "tools/check_docs.py")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("documentation check failed after checkpoint update: %w", err)
	}
	return nil
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
			if _, err := os.Stat(filepath.Join(dir, checkpointSourcePath)); err == nil {
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
	return bumpWithValidation(root, next, gate, nil)
}

func bumpWithValidation(root, next, gate string, validate func() error) error {
	updates, err := planBump(root, next, gate)
	if err != nil {
		return err
	}
	return applyUpdates(updates, writeAtomicMode, validate)
}

func planBump(root, next, gate string) ([]fileUpdate, error) {
	gate = strings.TrimSpace(gate)
	if gate == "" || strings.ContainsAny(gate, "\r\n|") {
		return nil, errors.New("gate name must be non-empty and must not contain newlines or '|'")
	}
	nextMinor, err := parseCheckpoint(next)
	if err != nil {
		return nil, err
	}

	sourcePath := filepath.Join(root, checkpointSourcePath)
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", checkpointSourcePath, err)
	}
	source, err := parseCheckpointSource(sourceData)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", checkpointSourcePath, err)
	}
	if err := validateMirrors(root, source); err != nil {
		return nil, err
	}
	currentMinor, err := parseCheckpoint(source.Current)
	if err != nil {
		return nil, fmt.Errorf("invalid current checkpoint %q: %w", source.Current, err)
	}
	if nextMinor != currentMinor+1 {
		return nil, fmt.Errorf("next checkpoint must be v0.%d; got %s", currentMinor+1, next)
	}

	contents := map[string][]byte{}
	for _, a := range authorities {
		data, err := os.ReadFile(filepath.Join(root, a.path))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", a.path, err)
		}
		data = a.current.ReplaceAllFunc(data, func(match []byte) []byte {
			return bytes.Replace(match, []byte(source.Current), []byte(next), 1)
		})
		contents[a.path] = data
	}

	for _, path := range []string{"docs/status/versioning-and-release-status.md", "docs/status/versioning-and-release-status.ja.md"} {
		data := contents[path]
		rowRE := regexp.MustCompile(`(?m)^\|\s*` + regexp.QuoteMeta(source.Current) + `\s*\|[^\n]*$`)
		loc := rowRE.FindIndex(data)
		if loc == nil {
			return nil, fmt.Errorf("%s: current checkpoint row %s not found", path, source.Current)
		}
		state := "✅ implemented"
		if strings.HasSuffix(path, ".ja.md") {
			state = "実装済み"
		}
		row := fmt.Sprintf("\n| %s | %s | %s |", next, gate, state)
		data = append(data[:loc[1]], append([]byte(row), data[loc[1]:]...)...)
		contents[path] = data
	}

	source.Current = next
	source.Milestones = append(source.Milestones, milestone{Version: next, Gate: gate, Minor: nextMinor})
	contents[checkpointSourcePath] = renderCheckpointSource(source)

	generatedPath := "internal/buildinfo/checkpoint_generated.go"
	generated, err := os.ReadFile(filepath.Join(root, generatedPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", generatedPath, err)
	}
	generatedRE := regexp.MustCompile(`const GeneratedCheckpoint = "v0\.\d+"`)
	if len(generatedRE.FindAll(generated, -1)) != 1 {
		return nil, fmt.Errorf("%s: expected exactly one generated checkpoint constant", generatedPath)
	}
	contents[generatedPath] = generatedRE.ReplaceAll(generated, []byte("const GeneratedCheckpoint = \""+next+"\""))

	paths := make([]string, 0, len(contents))
	for path := range contents {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	updates := make([]fileUpdate, 0, len(paths))
	for _, path := range paths {
		fullPath := filepath.Join(root, path)
		original, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read original %s: %w", path, err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		updates = append(updates, fileUpdate{
			Path:     fullPath,
			Original: original,
			Updated:  contents[path],
			Mode:     info.Mode(),
		})
	}
	return updates, nil
}

func applyUpdates(updates []fileUpdate, write writeFileFunc, validate func() error) error {
	applied := make([]fileUpdate, 0, len(updates))
	for _, update := range updates {
		if err := write(update.Path, update.Updated, update.Mode); err != nil {
			primary := fmt.Errorf("write %s: %w", update.Path, err)
			return rollbackUpdates(primary, applied, write)
		}
		applied = append(applied, update)
	}
	if validate != nil {
		if err := validate(); err != nil {
			return rollbackUpdates(err, applied, write)
		}
	}
	return nil
}

func rollbackUpdates(primary error, applied []fileUpdate, write writeFileFunc) error {
	var rollbackErrors []error
	for i := len(applied) - 1; i >= 0; i-- {
		update := applied[i]
		if err := write(update.Path, update.Original, update.Mode); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", update.Path, err))
		}
	}
	if len(rollbackErrors) == 0 {
		return primary
	}
	return errors.Join(primary, fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrors...)))
}

func parseCheckpointSource(data []byte) (checkpointSource, error) {
	var source checkpointSource
	seenSchema := false
	seenCurrent := false
	seenMilestones := false
	seenVersions := map[string]struct{}{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if line == "schema: 1" {
			if seenSchema {
				return checkpointSource{}, fmt.Errorf("line %d: duplicate schema", lineNumber)
			}
			seenSchema = true
			source.Schema = 1
			continue
		}
		if match := currentLineRE.FindStringSubmatch(line); len(match) == 2 {
			if seenCurrent {
				return checkpointSource{}, fmt.Errorf("line %d: duplicate current", lineNumber)
			}
			value, err := strconv.Unquote(match[1])
			if err != nil {
				return checkpointSource{}, fmt.Errorf("line %d: invalid current: %w", lineNumber, err)
			}
			seenCurrent = true
			source.Current = value
			continue
		}
		if line == "milestones:" {
			if seenMilestones {
				return checkpointSource{}, fmt.Errorf("line %d: duplicate milestones mapping", lineNumber)
			}
			seenMilestones = true
			continue
		}
		if match := milestoneLineRE.FindStringSubmatch(line); len(match) == 3 {
			if !seenMilestones {
				return checkpointSource{}, fmt.Errorf("line %d: milestone appears before milestones mapping", lineNumber)
			}
			version := match[1]
			if _, exists := seenVersions[version]; exists {
				return checkpointSource{}, fmt.Errorf("line %d: duplicate milestone %s", lineNumber, version)
			}
			minor, err := parseCheckpoint(version)
			if err != nil {
				return checkpointSource{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			gate, err := strconv.Unquote(match[2])
			if err != nil {
				return checkpointSource{}, fmt.Errorf("line %d: invalid gate string: %w", lineNumber, err)
			}
			gate = strings.TrimSpace(gate)
			if gate == "" || strings.ContainsAny(gate, "\r\n|") {
				return checkpointSource{}, fmt.Errorf("line %d: invalid gate name", lineNumber)
			}
			seenVersions[version] = struct{}{}
			source.Milestones = append(source.Milestones, milestone{Version: version, Gate: gate, Minor: minor})
			continue
		}
		return checkpointSource{}, fmt.Errorf("line %d: unsupported YAML syntax %q", lineNumber, line)
	}
	if err := scanner.Err(); err != nil {
		return checkpointSource{}, err
	}
	if !seenSchema || source.Schema != 1 {
		return checkpointSource{}, errors.New("schema: 1 is required")
	}
	if !seenCurrent {
		return checkpointSource{}, errors.New("current is required")
	}
	if !seenMilestones || len(source.Milestones) == 0 {
		return checkpointSource{}, errors.New("at least one milestone is required")
	}
	for i, item := range source.Milestones {
		expected := i + 1
		if item.Minor != expected || item.Version != fmt.Sprintf("v0.%d", expected) {
			return checkpointSource{}, fmt.Errorf("milestones must be contiguous from v0.1; position %d is %s", expected, item.Version)
		}
	}
	latest := source.Milestones[len(source.Milestones)-1].Version
	if source.Current != latest {
		return checkpointSource{}, fmt.Errorf("current %s must equal newest milestone %s", source.Current, latest)
	}
	return source, nil
}

func renderCheckpointSource(source checkpointSource) []byte {
	var out strings.Builder
	out.WriteString("# Hacocoon development-checkpoint numbering and gate identity.\n")
	out.WriteString("# This intentionally uses a constrained YAML subset parsed by repository tooling.\n")
	out.WriteString("schema: 1\n")
	fmt.Fprintf(&out, "current: %s\n", strconv.Quote(source.Current))
	out.WriteString("milestones:\n")
	for _, item := range source.Milestones {
		fmt.Fprintf(&out, "  %s: %s\n", strconv.Quote(item.Version), strconv.Quote(item.Gate))
	}
	return []byte(out.String())
}

func validateMirrors(root string, source checkpointSource) error {
	for _, a := range authorities {
		data, err := os.ReadFile(filepath.Join(root, a.path))
		if err != nil {
			return fmt.Errorf("read %s: %w", a.path, err)
		}
		matches := a.current.FindAllSubmatch(data, -1)
		if len(matches) != 1 {
			return fmt.Errorf("%s: expected exactly one current checkpoint declaration, found %d", a.path, len(matches))
		}
		if value := string(matches[0][1]); value != source.Current {
			return fmt.Errorf("%s: current checkpoint mirror %s disagrees with %s current %s", a.path, value, checkpointSourcePath, source.Current)
		}
	}

	rowRE := regexp.MustCompile(`(?m)^\|\s*(v0\.\d+)\s*\|\s*([^|]+?)\s*\|`)
	for _, path := range []string{"docs/status/versioning-and-release-status.md", "docs/status/versioning-and-release-status.ja.md"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rows := rowRE.FindAllSubmatch(data, -1)
		if len(rows) != len(source.Milestones) {
			return fmt.Errorf("%s: expected %d checkpoint rows from %s, found %d", path, len(source.Milestones), checkpointSourcePath, len(rows))
		}
		for i, row := range rows {
			version := string(row[1])
			gate := strings.TrimSpace(string(row[2]))
			expected := source.Milestones[i]
			if version != expected.Version || gate != expected.Gate {
				return fmt.Errorf("%s: row %d is %s / %q; expected %s / %q from %s", path, i+1, version, gate, expected.Version, expected.Gate, checkpointSourcePath)
			}
		}
	}

	generatedPath := "internal/buildinfo/checkpoint_generated.go"
	generated, err := os.ReadFile(filepath.Join(root, generatedPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", generatedPath, err)
	}
	generatedRE := regexp.MustCompile(`const GeneratedCheckpoint = "(v0\.\d+)"`)
	matches := generatedRE.FindAllSubmatch(generated, -1)
	if len(matches) != 1 {
		return fmt.Errorf("%s: expected exactly one generated checkpoint constant", generatedPath)
	}
	if value := string(matches[0][1]); value != source.Current {
		return fmt.Errorf("%s: generated checkpoint %s disagrees with %s current %s", generatedPath, value, checkpointSourcePath, source.Current)
	}
	return nil
}

func parseCheckpoint(value string) (int, error) {
	value = strings.TrimSpace(value)
	match := checkpointRE.FindStringSubmatch(value)
	if len(match) != 2 {
		return 0, fmt.Errorf("checkpoint must match v0.N: %q", value)
	}
	minor, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	if minor < 1 || value != fmt.Sprintf("v0.%d", minor) {
		return 0, fmt.Errorf("checkpoint must use canonical v0.N numbering from v0.1: %q", value)
	}
	return minor, nil
}

func writeAtomicMode(path string, data []byte, mode os.FileMode) error {
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
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
