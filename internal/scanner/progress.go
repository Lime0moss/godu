package scanner

import "time"

// Progress reports scanning progress.
type Progress struct {
	// CurrentPath is the directory currently being scanned.
	CurrentPath string
	// FilesScanned is the total files scanned so far.
	FilesScanned int64
	// DirsScanned is the total directories scanned so far.
	DirsScanned int64
	// BytesFound is the total bytes found so far.
	BytesFound int64
	// Errors is the count of errors encountered.
	Errors int64
	// Done indicates scanning is complete.
	Done bool
	// StartTime is when the scan began.
	StartTime time.Time
	// Duration is elapsed time.
	Duration time.Duration
}

// ItemsPerSecond returns the scan rate.
func (p Progress) ItemsPerSecond() float64 {
	if p.Duration.Seconds() == 0 {
		return 0
	}
	return float64(p.FilesScanned+p.DirsScanned) / p.Duration.Seconds()
}
