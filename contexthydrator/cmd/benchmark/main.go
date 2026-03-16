// benchmark measures the latency difference between:
//   - Cold path: reading directly from the upstream backend service
//   - Hot path:  reading from Redis cache (after hydration)
//
// Usage:
//
//	go run ./cmd/benchmark [flags]
//	make bench
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	hydratorURL := flag.String("hydrator", "http://localhost:8080", "context-hydrator base URL")
	backendURL := flag.String("backend", "http://localhost:9000", "mock backend base URL")
	userID := flag.String("user", "bench-user-1", "user ID to test with")
	resource := flag.String("resource", "profile", "resource to benchmark (profile|preferences|permissions|resources)")
	n := flag.Int("n", 100, "number of requests per path")
	flag.Parse()

	client := &http.Client{Timeout: 10 * time.Second}

	fmt.Println(banner())
	fmt.Printf("  hydrator : %s\n", *hydratorURL)
	fmt.Printf("  backend  : %s\n", *backendURL)
	fmt.Printf("  user_id  : %s\n", *userID)
	fmt.Printf("  resource : %s\n", *resource)
	fmt.Printf("  requests : %d per path\n\n", *n)

	// ── Step 1: Warm-up hydration ────────────────────────────────────────────
	fmt.Print("[ 1/3 ] Triggering hydration ... ")
	if err := triggerHydration(client, *hydratorURL, *userID); err != nil {
		fatalf("hydration failed: %v\n", err)
	}
	// Wait for fire-and-forget goroutine to finish (backend latency + Redis write)
	time.Sleep(500 * time.Millisecond)
	fmt.Println("done")

	// Confirm cache is warm
	fmt.Print("[ 2/3 ] Verifying cache is warm ... ")
	if err := verifyCache(client, *hydratorURL, *userID, *resource); err != nil {
		fatalf("\n        cache miss — is Redis running and hydration succeeding?\n        error: %v\n", err)
	}
	fmt.Print("done\n\n")

	// ── Step 2: Benchmark cold path (direct backend) ─────────────────────────
	fmt.Printf("[ 3/3 ] Benchmarking %d requests each ...\n\n", *n)

	coldURL := fmt.Sprintf("%s/users/%s/%s", *backendURL, *userID, *resource)
	coldSamples := measure(client, coldURL, *n, "  cold (backend) ")

	hotURL := fmt.Sprintf("%s/data/%s/%s", *hydratorURL, *userID, *resource)
	hotSamples := measure(client, hotURL, *n, "  hot  (cache)   ")

	// ── Step 3: Print report ─────────────────────────────────────────────────
	printReport(coldSamples, hotSamples)
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func triggerHydration(client *http.Client, baseURL, userID string) error {
	payload := map[string]string{
		"user_id":       userID,
		"session_token": "bench-session",
	}
	b, _ := json.Marshal(payload)
	encoded := base64.StdEncoding.EncodeToString(b)

	body := fmt.Sprintf(`{"cookie":"%s"}`, encoded)
	resp, err := client.Post(baseURL+"/hydrate", "application/json", strings.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func verifyCache(client *http.Client, baseURL, userID, resource string) error {
	resp, err := client.Get(fmt.Sprintf("%s/data/%s/%s", baseURL, userID, resource))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ── Measurement ───────────────────────────────────────────────────────────────

func measure(client *http.Client, url string, n int, label string) []float64 {
	samples := make([]float64, 0, n)

	for i := 0; i < n; i++ {
		start := time.Now()
		resp, err := client.Get(url)
		elapsed := time.Since(start)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nrequest error: %v\n", err)
			continue
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
		samples = append(samples, float64(elapsed.Microseconds())/1000.0) // ms

		pct := float64(i+1) / float64(n)
		bar := int(pct * 30)
		fmt.Printf("\r%s [%s%s] %d/%d", label,
			strings.Repeat("█", bar), strings.Repeat("░", 30-bar), i+1, n)
	}
	fmt.Println()
	return samples
}

// ── Statistics & report ───────────────────────────────────────────────────────

type stats struct {
	min, max, mean, p50, p95, p99 float64
	n                             int
}

func computeStats(samples []float64) stats {
	if len(samples) == 0 {
		return stats{}
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)

	sum := 0.0
	for _, v := range sorted {
		sum += v
	}

	return stats{
		n:    len(sorted),
		min:  sorted[0],
		max:  sorted[len(sorted)-1],
		mean: sum / float64(len(sorted)),
		p50:  percentile(sorted, 50),
		p95:  percentile(sorted, 95),
		p99:  percentile(sorted, 99),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (idx-float64(lo))*(sorted[hi]-sorted[lo])
}

func printReport(cold, hot []float64) {
	cs := computeStats(cold)
	hs := computeStats(hot)

	speedup := func(c, h float64) string {
		if h == 0 {
			return "N/A"
		}
		return fmt.Sprintf("%.0fx faster", c/h)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 62))
	fmt.Printf("  %-12s  %8s  %8s  %8s  %8s  %8s\n",
		"", "mean", "p50", "p95", "p99", "max")
	fmt.Println(strings.Repeat("─", 62))
	fmt.Printf("  %-12s  %7.2fms  %7.2fms  %7.2fms  %7.2fms  %7.2fms\n",
		"cold (backend)", cs.mean, cs.p50, cs.p95, cs.p99, cs.max)
	fmt.Printf("  %-12s  %7.2fms  %7.2fms  %7.2fms  %7.2fms  %7.2fms\n",
		"hot (cache)", hs.mean, hs.p50, hs.p95, hs.p99, hs.max)
	fmt.Println(strings.Repeat("─", 62))
	fmt.Printf("  %-12s  %8s  %8s  %8s  %8s\n",
		"improvement",
		speedup(cs.mean, hs.mean),
		speedup(cs.p50, hs.p50),
		speedup(cs.p95, hs.p95),
		speedup(cs.p99, hs.p99),
	)
	fmt.Println(strings.Repeat("─", 62))
	fmt.Printf("\n  cold samples: %d   hot samples: %d\n\n", cs.n, hs.n)
}

func banner() string {
	return `
╔══════════════════════════════════════════════════╗
║        Context Hydrator — Latency Benchmark      ║
║  cold = direct backend call                      ║
║  hot  = Redis cache read (post-hydration)        ║
╚══════════════════════════════════════════════════╝`
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format, args...)
	os.Exit(1)
}
