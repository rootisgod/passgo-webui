package multipass

// HostResources contains the host machine's total CPU and memory capacity.
type HostResources struct {
	TotalMemoryMB int64 `json:"total_memory_mb"`
	TotalCPUs     int   `json:"total_cpus"`
}
