package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CleanupService struct {
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	dirPath   string
	isRunning bool
	mu        sync.Mutex
}

func NewCleanupService(dirPath string) *CleanupService {
	ctx, cancel := context.WithCancel(context.Background())
	return &CleanupService{
		interval: time.Hour,
		ctx:      ctx,
		cancel:   cancel,
		dirPath:  dirPath,
	}
}

// trigger immediate cleanup
func (cs *CleanupService) CleanupNow() error {
	return cs.performCleanup()
}

func (cs *CleanupService) Start() {
	cs.mu.Lock()
	if cs.isRunning {
		cs.mu.Unlock()
		return
	}
	cs.isRunning = true
	cs.mu.Unlock()

	go cs.run()
}

func (cs *CleanupService) run() {
	ticker := time.NewTicker(cs.interval)
	defer ticker.Stop()

	log.Printf("Cleanup service started - will clean files older than 1 hour in %s\n", cs.dirPath)

	for {
		select {
		case <-cs.ctx.Done():
			log.Println("Cleanup service stopped")
			return
		case <-ticker.C:
			if err := cs.performCleanup(); err != nil {
				log.Printf("Cleanup error: %v\n", err)
			}
		}
	}
}

func (cs *CleanupService) performCleanup() error {
	if _, err := os.Stat(cs.dirPath); os.IsNotExist(err) {
		return nil
	}

	files, err := os.ReadDir(cs.dirPath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}

	thresholdTime := time.Now().Add(-time.Hour)

	// Small BUFFER CHANNEL as semaphore to limit concurrent deletions
	semaphore := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			log.Printf("Warning: couldn't get info for file %s: %v\n", file.Name(), err)
			continue
		}

		if info.ModTime().Before(thresholdTime) {
			wg.Add(1)
			go func(f os.DirEntry) {
				defer wg.Done()
				semaphore <- struct{}{}        // Acquire
				defer func() { <-semaphore }() // Release

				filePath := filepath.Join(cs.dirPath, f.Name())
				if err := os.Remove(filePath); err != nil {
					log.Printf("Warning: couldn't delete file %s: %v\n", f.Name(), err)
				}
			}(file)
		}
	}

	wg.Wait()
	return nil
}

func (cs *CleanupService) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.isRunning {
		cs.cancel()
		cs.isRunning = false
	}
}
