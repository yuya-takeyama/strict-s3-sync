package planner

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// parallelGatherLocalFiles implements parallel file system traversal using worker pool pattern
func (p *FSToS3Planner) parallelGatherLocalFiles(basePath string, excludes []string) ([]ItemMetadata, error) {
	// ワーカー数の設定（デフォルト8、最大32）
	workerCount := 8
	if envWorkers := os.Getenv("STRICT_S3_SYNC_WORKERS"); envWorkers != "" {
		if count, err := strconv.Atoi(envWorkers); err == nil && count > 0 {
			workerCount = count
			if workerCount > 32 {
				workerCount = 32
			}
		}
	}

	type dirTask struct {
		path string
	}

	// チャンネルの作成
	dirQueue := make(chan dirTask, 1000)

	// 結果の収集
	var items []ItemMetadata
	var itemsMutex sync.Mutex
	var resultErr error
	var errOnce sync.Once

	// ディレクトリ処理の待機グループ
	var dirWg sync.WaitGroup

	// ワーカープール
	var workerWg sync.WaitGroup
	workerWg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer workerWg.Done()

			for task := range dirQueue {
				entries, err := os.ReadDir(task.path)
				if err != nil {
					// アクセス権限エラーなどは警告として扱い、処理を継続
					if !os.IsPermission(err) {
						errOnce.Do(func() {
							resultErr = err
						})
					}
					dirWg.Done() // このディレクトリの処理完了
					continue
				}

				for _, entry := range entries {
					fullPath := filepath.Join(task.path, entry.Name())

					if entry.IsDir() {
						// 新しいディレクトリを発見
						dirWg.Add(1)

						// ノンブロッキングで送信を試みる
						select {
						case dirQueue <- dirTask{path: fullPath}:
							// 成功
						default:
							// キューが満杯の場合はブロッキング送信
							go func(path string) {
								dirQueue <- dirTask{path: path}
							}(fullPath)
						}
						continue
					}

					// ファイルの処理
					info, err := entry.Info()
					if err != nil {
						continue
					}

					relPath, err := filepath.Rel(basePath, fullPath)
					if err != nil {
						continue
					}

					relPath = filepath.ToSlash(relPath)

					excluded, err := IsExcluded(relPath, excludes)
					if err != nil {
						continue
					}
					if excluded {
						continue
					}

					itemsMutex.Lock()
					items = append(items, ItemMetadata{
						Path:    relPath,
						Size:    info.Size(),
						ModTime: info.ModTime(),
					})
					itemsMutex.Unlock()
				}

				dirWg.Done() // このディレクトリの処理完了
			}
		}()
	}

	// 初期ディレクトリを追加
	dirWg.Add(1)
	dirQueue <- dirTask{path: basePath}

	// 全てのディレクトリの処理が完了するまで待機
	go func() {
		dirWg.Wait()
		close(dirQueue)
	}()

	// ワーカーの完了を待つ
	workerWg.Wait()

	if resultErr != nil {
		return nil, resultErr
	}

	// 結果をパスでソート
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})

	return items, nil
}
