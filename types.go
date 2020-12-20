package container_info

const (
PodNamespaceLabelKey  = "io.kubernetes.pod.namespace"
PodNameLabelKey       = "io.kubernetes.pod.name"
PodCgroupNamePrefix   = "pod"
)

type CgroupName []string


var (
	containerRoot = NewCgroupName([]string{}, "kubepods")
)

const (
	// systemdSuffix is the cgroup name suffix for systemd
	systemdSuffix string = ".slice"
)

const (
	CGROUP_BASE  = "/sys/fs/cgroup/memory"
	CGROUP_PROCS = "cgroup.procs"
)

const(
	Danavgpu = "dana.894/gpu"
	Nvidiagpu = "nvidia.com/gpu"
)