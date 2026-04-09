package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

const maxHistoryEntries = 50

type historyEntry struct {
	Timestamp    string   `json:"timestamp"`
	ScheduleID   string   `json:"schedule_id"`
	ScheduleName string   `json:"schedule_name"`
	Action       string   `json:"action"`
	Targets      []string `json:"targets"`
	Result       string   `json:"result"` // "success", "partial", "failed", "no_targets"
	Errors       []string `json:"errors,omitempty"`
}

type scheduler struct {
	server   *Server
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	lastFire map[string]time.Time
	history  []historyEntry
}

func newScheduler(s *Server) *scheduler {
	return &scheduler{
		server:   s,
		lastFire: make(map[string]time.Time),
	}
}

func (sc *scheduler) start() {
	ctx, cancel := context.WithCancel(context.Background())
	sc.cancel = cancel
	sc.done = make(chan struct{})
	go sc.run(ctx)
}

func (sc *scheduler) stop() {
	if sc.cancel != nil {
		sc.cancel()
		<-sc.done
	}
}

func (sc *scheduler) run(ctx context.Context) {
	defer close(sc.done)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sc.tick(now)
		}
	}
}

func (sc *scheduler) tick(now time.Time) {
	hour, min, _ := now.Clock()
	weekday := int(now.Weekday())

	// Copy schedules and vmGroups under lock
	sc.server.groupMu.Lock()
	schedules := make([]config.Schedule, len(sc.server.cfg.Schedules))
	copy(schedules, sc.server.cfg.Schedules)
	vmGroups := make(map[string]string, len(sc.server.cfg.VMGroups))
	for k, v := range sc.server.cfg.VMGroups {
		vmGroups[k] = v
	}
	sc.server.groupMu.Unlock()

	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		if !sc.timeMatches(sched, hour, min, weekday) {
			continue
		}
		if sc.alreadyFired(sched.ID, now) {
			continue
		}
		sc.markFired(sched.ID, now)
		sc.execute(sched, vmGroups)
	}
}

func (sc *scheduler) timeMatches(sched config.Schedule, hour, min, weekday int) bool {
	parts := strings.Split(sched.Time, ":")
	if len(parts) != 2 {
		return false
	}
	schedHour, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	schedMin, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	if schedHour != hour || schedMin != min {
		return false
	}
	for _, d := range sched.Days {
		if d == weekday {
			return true
		}
	}
	return false
}

func (sc *scheduler) alreadyFired(id string, now time.Time) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	last, ok := sc.lastFire[id]
	if !ok {
		return false
	}
	return last.Truncate(time.Minute).Equal(now.Truncate(time.Minute))
}

func (sc *scheduler) markFired(id string, now time.Time) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.lastFire[id] = now.Truncate(time.Minute)
}

func (sc *scheduler) runNow(id string) error {
	sc.server.groupMu.Lock()
	sched, _ := sc.server.cfg.GetSchedule(id)
	if sched == nil {
		sc.server.groupMu.Unlock()
		return fmt.Errorf("schedule %q not found", id)
	}
	schedCopy := *sched
	vmGroups := make(map[string]string, len(sc.server.cfg.VMGroups))
	for k, v := range sc.server.cfg.VMGroups {
		vmGroups[k] = v
	}
	sc.server.groupMu.Unlock()

	sc.execute(schedCopy, vmGroups)
	return nil
}

func (sc *scheduler) resolveTargets(sched config.Schedule, vmGroups map[string]string) []string {
	if sched.Group != "" {
		var vms []string
		for vm, group := range vmGroups {
			if group == sched.Group {
				vms = append(vms, vm)
			}
		}
		return vms
	}
	return sched.VMs
}

func (sc *scheduler) addHistory(entry historyEntry) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.history = append(sc.history, entry)
	if len(sc.history) > maxHistoryEntries {
		sc.history = sc.history[len(sc.history)-maxHistoryEntries:]
	}
}

func (sc *scheduler) getHistory() []historyEntry {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Return reversed copy (most recent first)
	result := make([]historyEntry, len(sc.history))
	for i, h := range sc.history {
		result[len(sc.history)-1-i] = h
	}
	return result
}

func (sc *scheduler) execute(sched config.Schedule, vmGroups map[string]string) {
	targets := sc.resolveTargets(sched, vmGroups)
	if len(targets) == 0 {
		sc.server.logger.Warn("schedule has no targets", "id", sched.ID, "name", sched.Name)
		sc.addHistory(historyEntry{
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			ScheduleID:   sched.ID,
			ScheduleName: sched.Name,
			Action:       sched.Action,
			Result:       "no_targets",
		})
		sc.server.eventLog.EmitEvent("schedule", sched.Action, "scheduler", sched.Name, "no_targets", "")
		return
	}

	sc.server.logger.Info("schedule fired", "id", sched.ID, "name", sched.Name, "action", sched.Action, "targets", targets)

	var errors []string

	switch sched.Action {
	case "start":
		for _, vm := range targets {
			if err := sc.server.mp.StartVM(vm); err != nil {
				sc.server.logger.Error("scheduled start failed", "vm", vm, "schedule", sched.ID, "err", err)
				errors = append(errors, vm+": "+err.Error())
			}
		}
	case "stop":
		for _, vm := range targets {
			if err := sc.server.mp.StopVM(vm); err != nil {
				sc.server.logger.Error("scheduled stop failed", "vm", vm, "schedule", sched.ID, "err", err)
				errors = append(errors, vm+": "+err.Error())
			}
		}
	case "playbook":
		sc.server.ansibleRunner.enqueue(sched.Playbook, targets)
	}

	result := "success"
	if len(errors) > 0 {
		if len(errors) == len(targets) {
			result = "failed"
		} else {
			result = "partial"
		}
	}

	sc.addHistory(historyEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		ScheduleID:   sched.ID,
		ScheduleName: sched.Name,
		Action:       sched.Action,
		Targets:      targets,
		Result:       result,
		Errors:       errors,
	})

	detail := strings.Join(targets, ", ")
	if len(errors) > 0 {
		detail += "; errors: " + strings.Join(errors, "; ")
	}
	sc.server.eventLog.EmitEvent("schedule", sched.Action, "scheduler", sched.Name, result, detail)
}
