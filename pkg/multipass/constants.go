package multipass

const (
	DefaultUbuntuRelease = "24.04"
	DefaultCPUCores      = 2
	DefaultRAMMB         = 1024
	DefaultDiskGB        = 8
	MinCPUCores          = 1
	MinRAMMB             = 512
	MinDiskGB            = 1
	VMNamePrefix         = "VM-"
	VMNameRandomLength   = 4
)

var UbuntuReleases = []string{"24.04", "22.04", "20.04", "18.04", "daily"}
