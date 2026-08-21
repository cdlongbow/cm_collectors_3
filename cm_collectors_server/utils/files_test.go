package utils

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSortFilesByOrderFileNameAscNatural(t *testing.T) {
	files := []string{
		"D:/video/show/10.mp4",
		"D:/video/show/02.mp4",
		"D:/video/show/1.mp4",
		"D:/video/show/第11集.mp4",
		"D:/video/show/第3集.mp4",
	}

	actual := SortFilesByOrder(files, FileNameAsc)
	expected := []string{
		"D:/video/show/1.mp4",
		"D:/video/show/02.mp4",
		"D:/video/show/第3集.mp4",
		"D:/video/show/10.mp4",
		"D:/video/show/第11集.mp4",
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("SortFilesByOrder() = %#v, want %#v", actual, expected)
	}
}

func TestSortFilesByOrderFileSize(t *testing.T) {
	tempDir := t.TempDir()
	paths := []string{
		filepath.Join(tempDir, "10.mp4"),
		filepath.Join(tempDir, "2.mp4"),
		filepath.Join(tempDir, "1.mp4"),
	}
	sizes := []int{20, 10, 10}
	for i, filePath := range paths {
		if err := os.WriteFile(filePath, make([]byte, sizes[i]), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	ascending := SortFilesByOrder(append([]string(nil), paths...), FileSizeAsc)
	wantAscending := []string{paths[2], paths[1], paths[0]}
	if !reflect.DeepEqual(ascending, wantAscending) {
		t.Fatalf("FileSizeAsc = %#v, want %#v", ascending, wantAscending)
	}

	descending := SortFilesByOrder(append([]string(nil), paths...), FileSizeDesc)
	wantDescending := []string{paths[0], paths[2], paths[1]}
	if !reflect.DeepEqual(descending, wantDescending) {
		t.Fatalf("FileSizeDesc = %#v, want %#v", descending, wantDescending)
	}
}
