package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type ProductLink struct {
	ASIN      string    `json:"asin"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Price     string    `json:"price"`
	Status    string    `json:"status"` // pending, processing, completed, failed
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     string    `json:"error,omitempty"`
}

type LinkStorage struct {
	mu       sync.RWMutex
	links    map[string]*ProductLink
	filename string
}

func NewLinkStorage(filename string) (*LinkStorage, error) {
	ls := &LinkStorage{
		links:    make(map[string]*ProductLink),
		filename: filename,
	}
	
	// Load existing data if file exists
	if err := ls.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	
	return ls, nil
}

func (ls *LinkStorage) Add(link *ProductLink) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	if link.ASIN == "" {
		return fmt.Errorf("ASIN is required")
	}
	
	link.AddedAt = time.Now()
	link.UpdatedAt = time.Now()
	if link.Status == "" {
		link.Status = "pending"
	}
	
	ls.links[link.ASIN] = link
	return ls.save()
}

func (ls *LinkStorage) AddBatch(links []*ProductLink) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	for _, link := range links {
		if link.ASIN == "" {
			continue
		}
		
		link.AddedAt = time.Now()
		link.UpdatedAt = time.Now()
		if link.Status == "" {
			link.Status = "pending"
		}
		
		ls.links[link.ASIN] = link
	}
	
	return ls.save()
}

func (ls *LinkStorage) Get(asin string) (*ProductLink, bool) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	
	link, exists := ls.links[asin]
	return link, exists
}

func (ls *LinkStorage) GetPending() []*ProductLink {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	
	var pending []*ProductLink
	for _, link := range ls.links {
		if link.Status == "pending" {
			pending = append(pending, link)
		}
	}
	return pending
}

func (ls *LinkStorage) UpdateStatus(asin, status string, errorMsg string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	link, exists := ls.links[asin]
	if !exists {
		return fmt.Errorf("link not found: %s", asin)
	}
	
	link.Status = status
	link.UpdatedAt = time.Now()
	link.Error = errorMsg
	
	return ls.save()
}

func (ls *LinkStorage) GetStats() map[string]int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	
	stats := make(map[string]int)
	for _, link := range ls.links {
		stats[link.Status]++
	}
	stats["total"] = len(ls.links)
	return stats
}

func (ls *LinkStorage) save() error {
	data, err := json.MarshalIndent(ls.links, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to temp file first for atomicity
	tmpFile := ls.filename + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	
	// Rename to actual file
	return os.Rename(tmpFile, ls.filename)
}

func (ls *LinkStorage) Load() error {
	data, err := os.ReadFile(ls.filename)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, &ls.links)
}