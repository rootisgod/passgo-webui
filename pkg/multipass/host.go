package multipass

// HostResources contains the host machine's resource capacity and current usage.
type HostResources struct {
	TotalCPUs    int     `json:"total_cpus"`
	LoadAvg1     float64 `json:"load_avg_1"`
	LoadAvg5     float64 `json:"load_avg_5"`
	LoadAvg15    float64 `json:"load_avg_15"`
	TotalMemoryMB int64  `json:"total_memory_mb"`
	UsedMemoryMB  int64  `json:"used_memory_mb"`
	TotalDiskMB   int64  `json:"total_disk_mb"`
	UsedDiskMB    int64  `json:"used_disk_mb"`
}
