package sync

import "sync"

// CollectWithError collects the results of resChan and errChan and returns the
// results as array.
func CollectWithError[TRes any](resChan chan TRes, errChan chan error) ([]TRes, []error) {
	results := []TRes{}
	errors := []error{}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for r := range resChan {
			results = append(results, r)
		}
		wg.Done()
	}()
	go func() {
		for err := range errChan {
			errors = append(errors, err)
		}
		wg.Done()
	}()
	wg.Wait()
	return results, errors
}
