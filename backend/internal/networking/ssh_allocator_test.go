package networking

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
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

func TestSSHPortAllocator_ProviderReconciliationAndDeviceParsing(t *testing.T) {
	ctx := context.Background()

	// 1. Test ParseSSHProxyDevice unit cases
	t.Run("ParseSSHProxyDevice_Validation", func(t *testing.T) {
		// Valid Jesko SSH proxy device
		jeskoDev := map[string]string{
			"type":    "proxy",
			"listen":  "tcp:0.0.0.0:22100",
			"connect": "tcp:10.170.92.243:22",
		}
		info, ok := ParseSSHProxyDevice("ssh", jeskoDev)
		if !ok || info.PublicPort != 22100 || info.PrivateIP != "10.170.92.243" {
			t.Errorf("Failed to parse Jesko SSH proxy device: got %+v, ok=%v", info, ok)
		}

		// Non-SSH proxy device (port 80)
		httpDev := map[string]string{
			"type":    "proxy",
			"listen":  "tcp:0.0.0.0:80",
			"connect": "tcp:10.170.92.243:80",
		}
		if _, ok := ParseSSHProxyDevice("http", httpDev); ok {
			t.Errorf("Expected non-SSH port 80 to be ignored")
		}

		// Out of range SSH proxy (port 22099)
		lowPortDev := map[string]string{
			"type":    "proxy",
			"listen":  "tcp:0.0.0.0:22099",
			"connect": "tcp:10.170.92.243:22",
		}
		if _, ok := ParseSSHProxyDevice("ssh-low", lowPortDev); ok {
			t.Errorf("Expected port 22099 outside range 22100-22200 to be ignored")
		}

		// Out of range SSH proxy (port 22201)
		highPortDev := map[string]string{
			"type":    "proxy",
			"listen":  "tcp:0.0.0.0:22201",
			"connect": "tcp:10.170.92.243:22",
		}
		if _, ok := ParseSSHProxyDevice("ssh-high", highPortDev); ok {
			t.Errorf("Expected port 22201 outside range 22100-22200 to be ignored")
		}

		// Non-SSH destination port inside range (port 8080)
		nonSSHPortDev := map[string]string{
			"type":    "proxy",
			"listen":  "tcp:0.0.0.0:22105",
			"connect": "tcp:10.170.92.243:8080",
		}
		if _, ok := ParseSSHProxyDevice("web-alt", nonSSHPortDev); ok {
			t.Errorf("Expected non-22 destination port to be ignored")
		}

		// Disk device
		diskDev := map[string]string{
			"type": "disk",
			"path": "/",
		}
		if _, ok := ParseSSHProxyDevice("root", diskDev); ok {
			t.Errorf("Expected disk device to be ignored")
		}

		// NIC device
		nicDev := map[string]string{
			"type":    "nic",
			"network": "incusbr0",
		}
		if _, ok := ParseSSHProxyDevice("eth0", nicDev); ok {
			t.Errorf("Expected nic device to be ignored")
		}
	})

	// 2. Test Provider Instances Reconciliation & Lowest Available Port Selection
	t.Run("ReconcileProviderInstances_JeskoAndMultipleAllocations", func(t *testing.T) {
		tmpDir := t.TempDir()
		storeFile := filepath.Join(tmpDir, "ssh_allocations.json")
		store := NewFileSSHAllocationStore(storeFile)
		allocator := NewSSHPortAllocator(store, nil)

		// Mock live provider instances including Jesko (22100) and another instance (22102)
		mockInstances := []interfaces.Instance{
			{
				ID:   "jesko",
				Name: "jesko",
				Devices: map[string]map[string]string{
					"root": {"type": "disk", "path": "/"},
					"eth0": {"type": "nic", "network": "incusbr0"},
					"ssh":  {"type": "proxy", "listen": "tcp:0.0.0.0:22100", "connect": "tcp:10.170.92.243:22"},
					"web":  {"type": "proxy", "listen": "tcp:0.0.0.0:80", "connect": "tcp:10.170.92.243:80"},
				},
			},
			{
				ID:   "vps-nano-02",
				Name: "vps-nano-02",
				Devices: map[string]map[string]string{
					"eth0":      {"type": "nic", "network": "incusbr0"},
					"ssh-proxy": {"type": "proxy", "listen": "tcp:0.0.0.0:22102", "connect": "tcp:10.170.92.244:22"},
				},
			},
		}

		// Reconcile from provider instances
		allocator.ReconcileFromProviderInstances(ctx, mockInstances)

		// Verify Jesko 22100 is marked occupied
		jeskoAlloc, err := allocator.GetAllocation(ctx, "jesko")
		if err != nil || jeskoAlloc.PublicPort != 22100 {
			t.Fatalf("Expected Jesko port 22100 occupied, got alloc=%v, err=%v", jeskoAlloc, err)
		}

		// Verify vps-nano-02 22102 is marked occupied
		nanoAlloc, err := allocator.GetAllocation(ctx, "vps-nano-02")
		if err != nil || nanoAlloc.PublicPort != 22102 {
			t.Fatalf("Expected vps-nano-02 port 22102 occupied, got alloc=%v, err=%v", nanoAlloc, err)
		}

		// Duplicate / repeated reconciliation call must NOT create duplicate records
		allocator.ReconcileFromProviderInstances(ctx, mockInstances)
		allocs, _ := allocator.ListAllocations(ctx)
		if len(allocs) != 2 {
			t.Errorf("Expected exactly 2 allocations after duplicate reconciliation, got %d", len(allocs))
		}

		// First new allocation MUST pick lowest available port (22101, since 22100 is Jesko & 22102 is nano)
		newAlloc1, err := allocator.AllocatePort(ctx, "wl-new-01", "new-vps-01")
		if err != nil {
			t.Fatalf("AllocatePort failed: %v", err)
		}
		if newAlloc1.PublicPort != 22101 {
			t.Errorf("Expected lowest available port 22101, got %d", newAlloc1.PublicPort)
		}

		// Second new allocation MUST pick 22103 (since 22100, 22101, 22102 are occupied)
		newAlloc2, err := allocator.AllocatePort(ctx, "wl-new-02", "new-vps-02")
		if err != nil {
			t.Fatalf("AllocatePort failed: %v", err)
		}
		if newAlloc2.PublicPort != 22103 {
			t.Errorf("Expected next port 22103, got %d", newAlloc2.PublicPort)
		}
	})
}
