package networking

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestSSHPortAllocator_JeskoAndSequentialAllocation(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "ssh_allocations.json")

	store := NewFileSSHAllocationStore(storeFile)
	allocator := NewSSHPortAllocator(store, nil)
	ctx := context.Background()

	// 1. Jesko 22100 external reservation
	allocator.RegisterExternalPortReservation("wl-jesko", "jesko", 22100, "10.170.92.243")

	// 2. First new allocation MUST be 22101
	alloc1, err := allocator.AllocatePort(ctx, "wl-vps-01", "vps-01")
	if err != nil {
		t.Fatalf("AllocatePort wl-vps-01 failed: %v", err)
	}
	if alloc1.PublicPort != 22101 {
		t.Errorf("Expected first allocation to be 22101, got %d", alloc1.PublicPort)
	}

	// 3. Second allocation MUST be 22102
	alloc2, err := allocator.AllocatePort(ctx, "wl-vps-02", "vps-02")
	if err != nil {
		t.Fatalf("AllocatePort wl-vps-02 failed: %v", err)
	}
	if alloc2.PublicPort != 22102 {
		t.Errorf("Expected second allocation to be 22102, got %d", alloc2.PublicPort)
	}

	// 4. Reload from persistent store and check reservations survive restart
	store2 := NewFileSSHAllocationStore(storeFile)
	allocator2 := NewSSHPortAllocator(store2, nil)

	fetchedJesko, err := allocator2.GetAllocation(ctx, "wl-jesko")
	if err != nil || fetchedJesko.PublicPort != 22100 {
		t.Errorf("Expected reloaded Jesko port 22100, got %v, err=%v", fetchedJesko, err)
	}

	fetched1, err := allocator2.GetAllocation(ctx, "wl-vps-01")
	if err != nil || fetched1.PublicPort != 22101 {
		t.Errorf("Expected reloaded vps-01 port 22101, got %v, err=%v", fetched1, err)
	}
}

func TestSSHPortAllocator_CapacityExhaustion(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "ssh_allocations.json")

	store := NewFileSSHAllocationStore(storeFile)
	allocator := NewSSHPortAllocator(store, nil)
	ctx := context.Background()

	// Fill all 101 ports (22100-22200)
	for i := 0; i < 101; i++ {
		wlID := fmt.Sprintf("wl-fill-%d", i)
		alloc, err := allocator.AllocatePort(ctx, wlID, fmt.Sprintf("vps-%d", i))
		if err != nil {
			t.Fatalf("Failed to allocate port #%d: %v", i, err)
		}
		expectedPort := 22100 + i
		if alloc.PublicPort != expectedPort {
			t.Fatalf("Expected port %d, got %d", expectedPort, alloc.PublicPort)
		}
	}

	// 102nd allocation MUST fail with exact capacity error
	_, err := allocator.AllocatePort(ctx, "wl-overflow", "vps-overflow")
	if err == nil {
		t.Fatalf("Expected capacity failure, but allocation succeeded")
	}
	if err.Error() != "No public SSH ports are currently available." {
		t.Errorf("Expected exact error message 'No public SSH ports are currently available.', got '%s'", err.Error())
	}
}

func TestSSHPortAllocator_ConcurrentAllocation(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "ssh_allocations.json")

	store := NewFileSSHAllocationStore(storeFile)
	allocator := NewSSHPortAllocator(store, nil)
	ctx := context.Background()

	const numWorkers = 20
	var wg sync.WaitGroup
	portsChan := make(chan int, numWorkers)
	errChan := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			wlID := fmt.Sprintf("wl-concurrent-%d", workerID)
			alloc, err := allocator.AllocatePort(ctx, wlID, fmt.Sprintf("vps-conc-%d", workerID))
			if err != nil {
				errChan <- err
				return
			}
			portsChan <- alloc.PublicPort
		}(i)
	}

	wg.Wait()
	close(portsChan)
	close(errChan)

	for err := range errChan {
		t.Fatalf("Concurrent allocation encountered error: %v", err)
	}

	seenPorts := make(map[int]bool)
	for port := range portsChan {
		if port < 22100 || port > 22200 {
			t.Errorf("Port %d is outside valid pool 22100-22200", port)
		}
		if seenPorts[port] {
			t.Errorf("Duplicate port %d allocated concurrently!", port)
		}
		seenPorts[port] = true
	}

	if len(seenPorts) != numWorkers {
		t.Errorf("Expected %d unique ports, got %d", numWorkers, len(seenPorts))
	}
}

func TestSSHPortAllocator_ReleaseAndReuse(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "ssh_allocations.json")

	store := NewFileSSHAllocationStore(storeFile)
	allocator := NewSSHPortAllocator(store, nil)
	ctx := context.Background()

	allocator.RegisterExternalPortReservation("wl-jesko", "jesko", 22100, "10.170.92.243")

	alloc1, _ := allocator.AllocatePort(ctx, "wl-01", "vps-01") // 22101
	alloc2, _ := allocator.AllocatePort(ctx, "wl-02", "vps-02") // 22102

	if alloc1.PublicPort != 22101 || alloc2.PublicPort != 22102 {
		t.Fatalf("Setup ports unexpected: 01=%d, 02=%d", alloc1.PublicPort, alloc2.PublicPort)
	}

	// Release wl-01 (port 22101)
	if err := allocator.ReleasePort(ctx, "wl-01"); err != nil {
		t.Fatalf("ReleasePort failed: %v", err)
	}

	// Next allocation should re-use lowest available port (22101)
	alloc3, err := allocator.AllocatePort(ctx, "wl-03", "vps-03")
	if err != nil {
		t.Fatalf("AllocatePort wl-03 failed: %v", err)
	}
	if alloc3.PublicPort != 22101 {
		t.Errorf("Expected re-used port 22101, got %d", alloc3.PublicPort)
	}
}
