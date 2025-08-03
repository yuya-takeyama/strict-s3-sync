package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuya-takeyama/strict-s3-sync/pkg/logger"
)

func TestParallelGatherLocalFiles(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tmpDir := t.TempDir()

	// テストファイル構造を作成
	testFiles := []struct {
		path    string
		isDir   bool
		content string
	}{
		{"file1.txt", false, "content1"},
		{"file2.txt", false, "content2"},
		{"dir1", true, ""},
		{"dir1/file3.txt", false, "content3"},
		{"dir1/file4.txt", false, "content4"},
		{"dir1/subdir", true, ""},
		{"dir1/subdir/file5.txt", false, "content5"},
		{"dir2", true, ""},
		{"dir2/file6.txt", false, "content6"},
		{".hidden", false, "hidden"},
		{"dir1/.gitignore", false, "ignored"},
	}

	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.path)
		if tf.isDir {
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", path, err)
			}
		} else {
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create parent directory %s: %v", dir, err)
			}
			if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
				t.Fatalf("Failed to create file %s: %v", path, err)
			}
		}
	}

	// FSToS3Planner を作成
	planner := &FSToS3Planner{
		client: nil, // このテストではS3クライアントは使わない
		logger: &logger.SyncLogger{IsDryRun: false, IsQuiet: true},
	}

	tests := []struct {
		name         string
		excludes     []string
		wantFiles    []string
		wantExcluded []string
	}{
		{
			name:     "no excludes",
			excludes: []string{},
			wantFiles: []string{
				".hidden",
				"dir1/.gitignore",
				"dir1/file3.txt",
				"dir1/file4.txt",
				"dir1/subdir/file5.txt",
				"dir2/file6.txt",
				"file1.txt",
				"file2.txt",
			},
			wantExcluded: []string{},
		},
		{
			name:     "exclude hidden files",
			excludes: []string{".*", "**/.*"},
			wantFiles: []string{
				"dir1/file3.txt",
				"dir1/file4.txt",
				"dir1/subdir/file5.txt",
				"dir2/file6.txt",
				"file1.txt",
				"file2.txt",
			},
			wantExcluded: []string{".hidden", "dir1/.gitignore"},
		},
		{
			name:     "exclude specific directory",
			excludes: []string{"dir1/**"},
			wantFiles: []string{
				".hidden",
				"dir2/file6.txt",
				"file1.txt",
				"file2.txt",
			},
			wantExcluded: []string{
				"dir1/.gitignore",
				"dir1/file3.txt",
				"dir1/file4.txt",
				"dir1/subdir/file5.txt",
			},
		},
		{
			name:     "exclude by pattern",
			excludes: []string{"**/*.txt"},
			wantFiles: []string{
				".hidden",
				"dir1/.gitignore",
			},
			wantExcluded: []string{
				"dir1/file3.txt",
				"dir1/file4.txt",
				"dir1/subdir/file5.txt",
				"dir2/file6.txt",
				"file1.txt",
				"file2.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 並列版のgatherLocalFilesを実行
			items, err := planner.parallelGatherLocalFiles(tmpDir, tt.excludes)
			if err != nil {
				t.Fatalf("parallelGatherLocalFiles failed: %v", err)
			}

			// 取得したファイルのパスを抽出
			gotPaths := make([]string, len(items))
			for i, item := range items {
				gotPaths[i] = item.Path
			}

			// 期待するファイル数と一致するか確認
			if len(gotPaths) != len(tt.wantFiles) {
				t.Errorf("Got %d files, want %d files", len(gotPaths), len(tt.wantFiles))
				t.Errorf("Got: %v", gotPaths)
				t.Errorf("Want: %v", tt.wantFiles)
			}

			// 各ファイルが期待通りか確認
			for i, wantPath := range tt.wantFiles {
				if i >= len(gotPaths) {
					t.Errorf("Missing file: %s", wantPath)
					continue
				}
				if gotPaths[i] != wantPath {
					t.Errorf("File %d: got %s, want %s", i, gotPaths[i], wantPath)
				}
			}

			// ファイルのメタデータが正しく設定されているか確認
			for _, item := range items {
				if item.Size == 0 && item.Path != ".hidden" { // .hidden は空の可能性がある
					// サイズが0のファイルは内容が空でない限りエラー
					fullPath := filepath.Join(tmpDir, item.Path)
					content, _ := os.ReadFile(fullPath)
					if len(content) > 0 {
						t.Errorf("File %s has size 0 but contains content", item.Path)
					}
				}
				if item.ModTime.IsZero() {
					t.Errorf("File %s has zero ModTime", item.Path)
				}
			}
		})
	}
}

// 並列版と順次版の結果が一致することを確認するテスト
func TestParallelGatherLocalFilesConsistency(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tmpDir := t.TempDir()

	// より複雑なディレクトリ構造を作成
	dirs := []string{
		"a/b/c/d",
		"a/b/e",
		"f/g/h",
		"i",
	}

	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", path, err)
		}
		// 各ディレクトリに複数のファイルを作成
		for i := 0; i < 3; i++ {
			filename := filepath.Join(path, "file"+string(rune('a'+i))+".txt")
			content := "content of " + filename
			if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create file %s: %v", filename, err)
			}
		}
	}

	planner := &FSToS3Planner{
		client: nil,
		logger: &logger.SyncLogger{IsDryRun: false, IsQuiet: true},
	}

	// 順次版の実装（比較用）
	sequentialGather := func(basePath string, excludes []string) ([]ItemMetadata, error) {
		var items []ItemMetadata
		err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, err := filepath.Rel(basePath, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)
			excluded, err := IsExcluded(relPath, excludes)
			if err != nil {
				return err
			}
			if excluded {
				return nil
			}
			items = append(items, ItemMetadata{
				Path:    relPath,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
			return nil
		})
		return items, err
	}

	// 両方の実装を実行
	seqItems, err := sequentialGather(tmpDir, []string{})
	if err != nil {
		t.Fatalf("Sequential gather failed: %v", err)
	}

	parItems, err := planner.parallelGatherLocalFiles(tmpDir, []string{})
	if err != nil {
		t.Fatalf("Parallel gather failed: %v", err)
	}

	// 結果が同じ数のファイルを含むか確認
	if len(seqItems) != len(parItems) {
		t.Errorf("Different number of files: sequential=%d, parallel=%d", len(seqItems), len(parItems))
	}

	// パスでソート（両方とも同じ順序になるはず）
	seqPaths := make([]string, len(seqItems))
	for i, item := range seqItems {
		seqPaths[i] = item.Path
	}

	parPaths := make([]string, len(parItems))
	for i, item := range parItems {
		parPaths[i] = item.Path
	}

	// 同じファイルが含まれているか確認
	for i := range seqPaths {
		if i >= len(parPaths) {
			t.Errorf("Parallel missing file: %s", seqPaths[i])
			continue
		}
		if seqPaths[i] != parPaths[i] {
			t.Errorf("Mismatch at index %d: sequential=%s, parallel=%s", i, seqPaths[i], parPaths[i])
		}
	}
}
