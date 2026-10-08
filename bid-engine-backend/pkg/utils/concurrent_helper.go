package utils

import "sync"

// TaskResult 任务执行结果
type TaskResult struct {
	Index int         // 任务索引（保持顺序）
	Data  interface{} // 结果数据
	Error error       // 错误信息
}

// ExecuteConcurrent 并发执行任务
// taskCount: 任务总数
// concurrency: 最大并发数
// taskFunc: 任务处理函数，接收任务索引，返回结果和错误
func ExecuteConcurrent(
	taskCount int,
	concurrency int,
	taskFunc func(taskIndex int) (interface{}, error),
) []TaskResult {
	if taskCount == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	results := make(chan TaskResult, taskCount)
	var wg sync.WaitGroup

	// 启动所有任务
	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := taskFunc(index)
			results <- TaskResult{
				Index: index,
				Data:  data,
				Error: err,
			}
		}(i)
	}

	// 等待所有任务完成并关闭结果通道
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	taskResults := make([]TaskResult, 0, taskCount)
	for result := range results {
		taskResults = append(taskResults, result)
	}

	return taskResults
}
