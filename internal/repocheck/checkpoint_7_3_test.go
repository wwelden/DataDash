//go:build all || m7 || m7_3

// Checkpoint 7.3: ready to ship (repository checks)

package repocheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func root(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git checkout")
	}
	return strings.TrimSpace(string(out))
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	return string(b)
}

func TestCheckpoint7_3_Dockerfile(t *testing.T) {
	d := read(t, filepath.Join(root(t), "Dockerfile"))
	if !strings.Contains(d, "FROM") {
		t.Fatal("Dockerfile has no FROM line")
	}
	if strings.Count(d, "FROM") < 2 {
		t.Error("use a multi-stage build: one FROM stage with Go to compile, a second small stage to run the binary")
	}
	if strings.Contains(d, "APCA_API_SECRET_KEY=") {
		t.Error("the Dockerfile must not contain your secret; pass it as an environment variable at run time")
	}
}

func TestCheckpoint7_3_CIWorkflow(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(root(t), ".github", "workflows", "*.y*ml"))
	if len(files) == 0 {
		t.Fatal("no GitHub Actions workflow found in .github/workflows/")
	}
	all := ""
	for _, f := range files {
		all += read(t, f)
	}
	for _, want := range []string{"go vet", "go test", "-race", "-tags all"} {
		if !strings.Contains(all, want) {
			t.Errorf("the CI workflow should run %q", want)
		}
	}
}

func TestCheckpoint7_3_Readme(t *testing.T) {
	r := strings.ToLower(read(t, filepath.Join(root(t), "README.md")))
	for _, want := range []string{"architecture", "benchmark", "design decisions", "p99"} {
		if !strings.Contains(r, want) {
			t.Errorf("README.md should have a section or table mentioning %q", want)
		}
	}
}

var keyID = regexp.MustCompile(`\b[PA]K[A-Z0-9]{18}\b`)
var secretAssign = regexp.MustCompile(`APCA_API_SECRET_KEY\s*[=:]\s*["']?[A-Za-z0-9/+]{20,}`)

func TestCheckpoint7_3_NoSecretsInGit(t *testing.T) {
	r := root(t)
	tracked, err := exec.Command("git", "-C", r, "ls-files").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range strings.Split(strings.TrimSpace(string(tracked)), "\n") {
		if f == ".env" || strings.HasPrefix(f, ".env.") && f != ".env.example" {
			t.Errorf("%s is tracked by git; it holds secrets and must be ignored", f)
		}
	}
	history, err := exec.Command("git", "-C", r, "log", "--all", "-p", "--no-color").Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	for _, m := range keyID.FindAll(history, -1) {
		if strings.Contains(string(m), "BADBADBAD") { // the fake key used in checkpoint 5.3
			continue
		}
		t.Errorf("something that looks like an Alpaca key ID (%s...) is in git history. Rotate the key in the Alpaca dashboard.", m[:6])
		break
	}
	if secretAssign.Match(history) {
		t.Error("an APCA_API_SECRET_KEY value appears in git history. Rotate the key in the Alpaca dashboard.")
	}
}
