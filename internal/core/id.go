package core

import (
	"fmt"
	"time"
)

// NewCaseID builds a compact monotonic case identifier.
func NewCaseID(now time.Time) string {
	utc := now.UTC()
	return fmt.Sprintf("senate-%s", utc.Format("20060102-150405"))
}
