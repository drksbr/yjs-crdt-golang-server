package yupdate

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
)

const defaultUpdateAggregateWorkers = 5

type aggregateResult[T any] struct {
	index int
	value T
	err   error
}

type aggregateTask struct {
	index int
	data  []byte
}

func aggregatePayloadsInParallel[T any](
	ctx context.Context,
	updates [][]byte,
	workers int,
	extract func(context.Context, int, []byte) (T, error),
	reducer func(context.Context, []T) (T, error),
) (T, error) {
	var zero T
	if extract == nil {
		return zero, fmt.Errorf("extractor nao fornecido")
	}
	if reducer == nil {
		return zero, fmt.Errorf("reducer nao fornecido")
	}
	if len(updates) == 0 {
		return reducer(ctx, make([]T, 0))
	}

	workerCount := defaultUpdateAggregateWorkers
	if workers > 0 {
		workerCount = workers
	}
	workerCount = resolveWorkerCount(workerCount, len(updates))
	if workerCount == 0 {
		return zero, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan aggregateTask, len(updates))
	results := make(chan aggregateResult[T], len(updates))
	for index, update := range updates {
		jobs <- aggregateTask{
			index: index,
			data:  update,
		}
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if ctx.Err() != nil {
					return
				}

				value, err := runUpdateTaskSafely(ctx, task.index, task.data, extract)
				select {
				case results <- aggregateResult[T]{index: task.index, value: value, err: err}:
					if err != nil {
						cancel()
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]aggregateResult[T], 0, len(updates))
	for result := range results {
		out = append(out, result)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].index < out[j].index
	})

	entries := make([]T, 0, len(out))
	for _, result := range out {
		if result.err != nil {
			return zero, result.err
		}
		entries = append(entries, result.value)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	return reducer(ctx, entries)
}

func resolveWorkerCount(requested int, updatesCount int) int {
	if updatesCount <= 0 {
		return 0
	}
	if requested <= 0 {
		requested = 1
	}
	if requested > updatesCount {
		requested = updatesCount
	}
	if requested > runtime.GOMAXPROCS(0) {
		requested = runtime.GOMAXPROCS(0)
	}
	if requested < 1 {
		requested = 1
	}
	return requested
}

func runUpdateTaskSafely[T any](
	ctx context.Context,
	index int,
	data []byte,
	extract func(context.Context, int, []byte) (T, error),
) (value T, err error) {
	defer func() {
		if recoverValue := recover(); recoverValue != nil {
			err = fmt.Errorf("update[%d]: panic durante processamento: %v", index, recoverValue)
		}
	}()

	value, err = extract(ctx, index, data)
	if err != nil {
		err = fmt.Errorf("update[%d]: %w", index, err)
	}
	return value, err
}
