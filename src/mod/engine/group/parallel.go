package group

import (
	"runtime"
	"sync"
)

func parallelMap[T any](n int, fn func(i int) T) []T {
	out := make([]T, n)
	if n == 0 {
		return out
	}
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i] = fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}
