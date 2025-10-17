package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/queue"
	"github.com/maltedev/amazon-size-scraper/internal/ratelimit"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"
)

func main() {
	var (
		urls      = flag.String("urls", "", "Comma-separated list of Amazon product URLs to scrape")
		asins     = flag.String("asins", "", "Comma-separated list of Amazon ASINs to scrape")
		inputFile = flag.String("file", "", "File containing URLs or ASINs (one per line)")
		output    = flag.String("output", "stdout", "Output format: stdout, json, csv")
		headless  = flag.Bool("headless", true, "Run browser in headless mode")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting Amazon Size Scraper")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	browserOpts := &browser.Options{
		Headless:       *headless && cfg.Browser.Headless,
		Timeout:        cfg.Browser.Timeout,
		ViewportWidth:  cfg.Browser.ViewportWidth,
		ViewportHeight: cfg.Browser.ViewportHeight,
		AcceptLanguage: cfg.Browser.AcceptLanguage,
		TimezoneID:     cfg.Browser.TimezoneID,
		Locale:         cfg.Browser.Locale,
	}

	if len(cfg.Scraper.UserAgents) > 0 {
		browserOpts.UserAgent = cfg.Scraper.UserAgents[0]
	}

	b, err := browser.New(browserOpts)
	if err != nil {
		logger.Error("Failed to initialize browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	p := parser.NewAmazonParser()
	s := scraper.NewAmazonScraper(b, p, logger)

	taskQueue := queue.NewInMemoryQueue()
	defer taskQueue.Close()

	if err := loadTasks(taskQueue, *urls, *asins, *inputFile); err != nil {
		logger.Error("Failed to load tasks", "error", err)
		os.Exit(1)
	}

	if taskQueue.Size() == 0 {
		fmt.Println("No tasks to process. Use -urls, -asins, or -file to specify products to scrape.")
		flag.Usage()
		os.Exit(1)
	}

	rateLimiter := ratelimit.NewAdaptiveRateLimiter(
		cfg.Scraper.RateLimitMin,
		cfg.Scraper.RateLimitMax,
	)

	logger.Info("Starting scraping", "tasks", taskQueue.Size())

	for {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, exiting")
			return
		default:
		}

		task, err := taskQueue.Pop(ctx)
		if err != nil {
			if err == queue.ErrQueueEmpty || err == queue.ErrQueueClosed {
				logger.Info("Queue empty, finishing")
				break
			}
			logger.Error("Failed to get task from queue", "error", err)
			continue
		}

		if err := rateLimiter.Wait(ctx); err != nil {
			logger.Error("Rate limiter error", "error", err)
			continue
		}

		logger.Info("Processing task", "url", task.URL, "asin", task.ASIN)

		product, err := s.ScrapeByASIN(ctx, task.ASIN)
		if err != nil {
			logger.Error("Failed to scrape product", "asin", task.ASIN, "error", err)
			rateLimiter.RecordError()
			
			if task.Retries < cfg.Scraper.MaxRetries {
				task.Retries++
				taskQueue.Push(task)
				logger.Info("Retrying task", "asin", task.ASIN, "retry", task.Retries)
			}
			continue
		}

		rateLimiter.RecordSuccess()
		
		if err := outputResult(product, *output); err != nil {
			logger.Error("Failed to output result", "error", err)
		}
	}

	logger.Info("Scraping completed")
}

func loadTasks(q queue.Queue, urls, asins, inputFile string) error {
	var taskList []string

	if urls != "" {
		taskList = append(taskList, strings.Split(urls, ",")...)
	}

	if asins != "" {
		for _, asin := range strings.Split(asins, ",") {
			taskList = append(taskList, strings.TrimSpace(asin))
		}
	}

	if inputFile != "" {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				taskList = append(taskList, line)
			}
		}
	}

	for i, item := range taskList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		var task *queue.Task
		if strings.Contains(item, "amazon.de") {
			// Extract ASIN from URL using regex
			re := regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?amazon\.de/.*?/dp/([A-Z0-9]{10})`)
			matches := re.FindStringSubmatch(item)
			if len(matches) < 2 {
				continue
			}
			task = &queue.Task{
				ID:        fmt.Sprintf("task-%d", i),
				URL:       item,
				ASIN:      matches[1],
				Priority:  1,
				CreatedAt: time.Now(),
			}
		} else if len(item) == 10 {
			task = &queue.Task{
				ID:        fmt.Sprintf("task-%d", i),
				URL:       fmt.Sprintf("https://www.amazon.de/dp/%s", item),
				ASIN:      item,
				Priority:  1,
				CreatedAt: time.Now(),
			}
		}

		if task != nil {
			q.Push(task)
		}
	}

	return nil
}

func outputResult(product *models.Product, format string) error {
	switch format {
	case "json":
		// Implementation for JSON output
		fmt.Printf("%+v\n", product)
	case "csv":
		// Implementation for CSV output
		fmt.Printf("%s,%s,%.2fx%.2fx%.2f %s,%.2f %s\n",
			product.ASIN,
			product.Title,
			product.Dimensions.Length,
			product.Dimensions.Width,
			product.Dimensions.Height,
			product.Dimensions.Unit,
			product.Weight.Value,
			product.Weight.Unit,
		)
	default:
		fmt.Printf("Product: %s\n", product.Title)
		fmt.Printf("ASIN: %s\n", product.ASIN)
		fmt.Printf("Dimensions: %.2f x %.2f x %.2f %s\n",
			product.Dimensions.Length,
			product.Dimensions.Width,
			product.Dimensions.Height,
			product.Dimensions.Unit,
		)
		fmt.Printf("Weight: %.2f %s\n", product.Weight.Value, product.Weight.Unit)
		fmt.Printf("Price: %.2f %s\n", product.Price.Amount, product.Price.Currency)
		fmt.Println("---")
	}
	return nil
}