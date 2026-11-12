package model

type Collector string

const (
	CollectorG1       Collector = "G1"
	CollectorParallel Collector = "Parallel"
	CollectorCMS      Collector = "CMS"
	CollectorZGC      Collector = "ZGC"
	CollectorUnknown  Collector = "Unknown"
)

type JVMInfo struct {
	MajorVersion   int
	FullVersion    string
	Collector      Collector
	CPUs           int
	PhysicalMemory int64

	HeapMin     int64
	HeapInitial int64
	HeapMax     int64

	RegionSize     int64
	AlwaysPreTouch *bool
	RawFlags       []string
}
