package api

import (
	"sync"
	"time"
)

// LaunchStatus tracks the state of an async VM launch.
type LaunchStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "launching", "failed"
	Error   string `json:"error,omitempty"`
	Started time.Time `json:"started"`
}

// launchTracker keeps track of in-progress and recently-failed VM launches.
type launchTracker struct {
	mu       sync.Mutex
	launches map[string]*LaunchStatus
}

func newLaunchTracker() *launchTracker {
	return &launchTracker{launches: make(map[string]*LaunchStatus)}
}

func (lt *launchTracker) start(name string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.launches[name] = &LaunchStatus{
		Name:    name,
		Status:  "launching",
		Started: time.Now(),
	}
}

func (lt *launchTracker) complete(name string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	delete(lt.launches, name)
}

func (lt *launchTracker) fail(name string, err string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if ls, ok := lt.launches[name]; ok {
		ls.Status = "failed"
		ls.Error = err
	}
}

// dismiss removes a failed launch from the tracker (user acknowledged).
func (lt *launchTracker) dismiss(name string) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	delete(lt.launches, name)
}

func (lt *launchTracker) list() []LaunchStatus {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	// Clean up stale failed launches (older than 5 minutes)
	for name, ls := range lt.launches {
		if ls.Status == "failed" && time.Since(ls.Started) > 5*time.Minute {
			delete(lt.launches, name)
		}
	}

	result := make([]LaunchStatus, 0, len(lt.launches))
	for _, ls := range lt.launches {
		result = append(result, *ls)
	}
	return result
}
