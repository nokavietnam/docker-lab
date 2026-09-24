package crawler

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"

	"muaban-crawler/internal/config"
	"muaban-crawler/internal/model"
	"muaban-crawler/internal/storage"
)

type PageResult struct {
	Page       int
	TotalItems int
	Properties []model.Property
	Err        error
}

type Coordinator struct {
	cfg     *config.Config
	client  *HTTPClient
	storage *storage.Storage
}

func NewCoordinator(cfg *config.Config, client *HTTPClient, st *storage.Storage) *Coordinator {
	return &Coordinator{
		cfg:     cfg,
		client:  client,
		storage: st,
	}
}

// Run executes the distributed crawler pipeline with worker pools and memory-safe streaming
func (c *Coordinator) Run(ctx context.Context) error {
	categoryKey := c.cfg.CategoryPath
	startPage := c.cfg.StartPage

	// Check existing checkpoint
	cp, err := c.storage.GetCheckpoint(ctx, categoryKey)
	if err != nil {
		log.Printf("[Coordinator] Warning: could not read checkpoint: %v", err)
	} else if cp != nil && !c.cfg.IncrementalMode && cp.Status != "completed" && cp.LastPage >= startPage {
		// Non-incremental mode resumes from last page
		startPage = cp.LastPage + 1
		log.Printf("[Coordinator] Resuming full crawl from checkpoint: page %d (saved so far: %d items)", startPage, cp.TotalItems)
	} else if c.cfg.IncrementalMode {
		// Incremental mode always inspects from page 1 to capture new posts of the day
		startPage = 1
		log.Println("[Coordinator] Running in INCREMENTAL mode: scanning newest listings from page 1...")
	}

	tasksChan := make(chan int, c.cfg.Concurrency*2)
	resultsChan := make(chan PageResult, c.cfg.Concurrency*2)

	var discoveredTotalPages int64
	var earlyStopRequested int32

	// Step 1: Start Worker Pool
	var wg sync.WaitGroup
	for w := 1; w <= c.cfg.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for page := range tasksChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				pageURL := fmt.Sprintf("%s/%s?page=%d", c.cfg.BaseURL, c.cfg.CategoryPath, page)
				htmlBytes, fetchErr := c.client.FetchHTML(ctx, pageURL)
				if fetchErr != nil {
					select {
					case resultsChan <- PageResult{Page: page, Err: fetchErr}:
					case <-ctx.Done():
					}
					continue
				}

				items, totalItems, extractErr := ExtractListings(htmlBytes)
				select {
				case resultsChan <- PageResult{
					Page:       page,
					TotalItems: totalItems,
					Properties: items,
					Err:        extractErr,
				}:
				case <-ctx.Done():
					return
				}
			}
		}(w)
	}

	// Close resultsChan when all workers finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Step 2: Collector / DB Writer in background
	collectorDone := make(chan struct{})
	var totalSaved int64
	consecutiveExisting := 0

	go func() {
		defer close(collectorDone)
		var memStats runtime.MemStats

		for res := range resultsChan {
			if res.Err != nil {
				log.Printf("[Worker Error] Page %d: %v", res.Page, res.Err)
				continue
			}

			if res.TotalItems > 0 {
				calcPages := int64((res.TotalItems + 19) / 20)
				atomic.StoreInt64(&discoveredTotalPages, calcPages)
			}

			newCount := len(res.Properties)
			if c.cfg.IncrementalMode && len(res.Properties) > 0 {
				ids := make([]int64, len(res.Properties))
				for i, p := range res.Properties {
					ids[i] = p.ID
				}
				existingMap, qErr := c.storage.CheckExistingIDs(ctx, ids)
				existingCount := 0
				if qErr == nil {
					for _, id := range ids {
						if existingMap[id] {
							existingCount++
						}
					}
				}
				newCount = len(res.Properties) - existingCount

				// Detect if we hit the boundary of already-crawled items
				if existingCount == len(res.Properties) {
					consecutiveExisting += existingCount
					if consecutiveExisting >= c.cfg.MaxExistingThreshold {
						log.Printf("[Coordinator] Incremental Stop: Encountered %d consecutive already-crawled items at page %d. Halting crawl.",
							consecutiveExisting, res.Page)
						atomic.StoreInt32(&earlyStopRequested, 1)
					}
				} else {
					consecutiveExisting = 0
				}
			}

			// Save properties to DB (idempotent ON CONFLICT DO UPDATE)
			if len(res.Properties) > 0 {
				if err := c.storage.SavePropertiesBatch(ctx, res.Properties); err != nil {
					log.Printf("[Storage Error] Failed to save batch for page %d: %v", res.Page, err)
					continue
				}
				totalSaved += int64(len(res.Properties))
			}

			// Update checkpoint
			pages := int(atomic.LoadInt64(&discoveredTotalPages))
			cp := &model.Checkpoint{
				CategoryKey: categoryKey,
				LastPage:    res.Page,
				TotalPages:  pages,
				TotalItems:  int(totalSaved),
				Status:      "running",
			}
			_ = c.storage.SaveCheckpoint(ctx, cp)

			// Staff Engineer Telemetry: monitor memory allocation & stats
			runtime.ReadMemStats(&memStats)
			heapAllocMB := float64(memStats.HeapAlloc) / 1024 / 1024
			sysMB := float64(memStats.Sys) / 1024 / 1024

			log.Printf("[Progress] Page %d/%d | Saved: %d (New: +%d, Existed: %d) | RAM Heap: %.2f MB, Sys: %.2f MB | Goroutines: %d",
				res.Page, pages, totalSaved, newCount, len(res.Properties)-newCount, heapAllocMB, sysMB, runtime.NumGoroutine())
		}
	}()

	// Step 3: Producer loop sending page tasks
	log.Printf("[Coordinator] Starting producer from page %d (MaxPages: %d, Concurrency: %d)", startPage, c.cfg.MaxPages, c.cfg.Concurrency)

	currentPage := startPage
	endPage := startPage + c.cfg.MaxPages - 1

produceLoop:
	for {
		// Stop if category has fewer total pages than requested
		maxAvailable := atomic.LoadInt64(&discoveredTotalPages)
		if maxAvailable > 0 && int64(currentPage) > maxAvailable {
			log.Printf("[Coordinator] Reached end of category pages (%d).", maxAvailable)
			break produceLoop
		}

		// Stop early in incremental mode when boundary is reached
		if atomic.LoadInt32(&earlyStopRequested) == 1 {
			log.Println("[Coordinator] Early stop signal detected. Halting task producer...")
			break produceLoop
		}

		if currentPage > endPage {
			break produceLoop
		}

		select {
		case <-ctx.Done():
			log.Println("[Coordinator] Cancellation signal received. Stopping task producer...")
			break produceLoop
		case tasksChan <- currentPage:
			currentPage++
		}
	}

	// Close tasksChan explicitly so workers exit and wg.Wait() unblocks
	close(tasksChan)

	// Wait for collector to finish writing in-flight batches
	<-collectorDone

	// Final checkpoint update
	finalCount, _ := c.storage.CountProperties(context.Background())
	log.Printf("[Coordinator] Pipeline drained successfully. Total properties in database: %d", finalCount)

	finalStatus := "completed"
	if ctx.Err() != nil {
		finalStatus = "interrupted"
	}

	_ = c.storage.SaveCheckpoint(context.Background(), &model.Checkpoint{
		CategoryKey: categoryKey,
		LastPage:    currentPage - 1,
		TotalItems:  int(finalCount),
		Status:      finalStatus,
	})

	return nil
}
