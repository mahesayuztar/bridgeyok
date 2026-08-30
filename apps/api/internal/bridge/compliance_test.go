package bridge

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var complianceEvidencePattern = regexp.MustCompile("`(Test[A-Za-z0-9_]+)`")

func TestWBFComplianceMatrix(t *testing.T) {
	t.Parallel()

	repositoryRoot := filepath.Clean("../../../..")
	matrixPath := filepath.Join(repositoryRoot, "docs/product/wbf-compliance-matrix.md")
	matrixData, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", matrixPath, err)
	}
	matrix := string(matrixData)
	if !strings.Contains(matrix, "Laws 73 and 89 revisions effective 1 January 2024") {
		t.Fatal("compliance matrix does not identify the effective WBF revision baseline")
	}

	testFunctions := repositoryTestFunctions(t, filepath.Join(repositoryRoot, "apps/api/internal"))
	statuses := map[string]struct{}{
		"mechanically-enforced": {},
		"director-judgement":    {},
		"not-applicable":        {},
	}
	lawCount := 0
	for _lineIndex, line := range strings.Split(matrix, "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		columns := strings.Split(line, "|")
		if len(columns) != 6 {
			continue
		}
		law, err := strconv.Atoi(strings.TrimSpace(columns[1]))
		if err != nil {
			continue
		}
		lawCount++
		if law != lawCount {
			t.Fatalf("matrix line %d has Law %d, want Law %d", _lineIndex+1, law, lawCount)
		}
		status := strings.TrimSpace(columns[3])
		if _, exists := statuses[status]; !exists {
			t.Fatalf("Law %d has unsupported status %q", law, status)
		}
		evidence := strings.TrimSpace(columns[4])
		if evidence == "" {
			t.Fatalf("Law %d has no boundary rationale or evidence", law)
		}
		matches := complianceEvidencePattern.FindAllStringSubmatch(evidence, -1)
		if status == "mechanically-enforced" && len(matches) == 0 {
			t.Fatalf("Law %d is mechanically enforced without executable test evidence", law)
		}
		if status != "mechanically-enforced" && !strings.Contains(evidence, "Product Contract") {
			t.Fatalf("Law %d boundary does not reference the product contract", law)
		}
		for _, match := range matches {
			if _, exists := testFunctions[match[1]]; !exists {
				t.Errorf("Law %d references missing test %s", law, match[1])
			}
		}
	}
	if lawCount != 93 {
		t.Fatalf("compliance matrix contains %d laws, want 93", lawCount)
	}
}

func repositoryTestFunctions(t *testing.T, root string) map[string]struct{} {
	t.Helper()

	testFunctions := make(map[string]struct{})
	functionPattern := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range functionPattern.FindAllStringSubmatch(string(data), -1) {
			testFunctions[match[1]] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%s) error = %v", root, err)
	}
	return testFunctions
}
