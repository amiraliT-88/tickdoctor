package model

import "time"

type GCType string

const (
	GCTypeYoung      GCType = "Young"
	GCTypeMixed      GCType = "Mixed"
	GCTypeFull       GCType = "Full"
	GCTypeRemark     GCType = "Remark"
	GCTypeCleanup    GCType = "Cleanup"
	GCTypeConcurrent GCType = "Concurrent"
	GCTypeUnknown    GCType = "Unknown"
)

type GCEvent struct {
	ID        uint64
	StartTime time.Time
	Uptime    time.Duration
	Duration  time.Duration
	Type      GCType
	Reason    string

	HeapBefore int64
	HeapAfter  int64
	HeapTotal  int64

	EdenBefore int64
	EdenAfter  int64
	EdenTotal  int64

	OldBefore int64
	OldAfter  int64
	OldTotal  int64

	HumongousCount int
	RawLines       []string
}

type LagEvent struct {
	Time        time.Time
	Duration    time.Duration
	TicksBehind int
	Source      string
	RawLine     string
}
