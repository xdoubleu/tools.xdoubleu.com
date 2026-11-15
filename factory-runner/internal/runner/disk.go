package runner

import (
	"context"
	"fmt"
	"syscall"
)

// LowDiskBytes is the free space below which the runner cleans up before it
// pulls an image, and tells the board when that did not help.
const (
	LowDiskBytes  = 10 << 30
	gigabyteShift = 30
)

// FreeBytes is the space an unprivileged user can still use on dir's volume.
func FreeBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, fmt.Errorf("free space of %s: %w", dir, err)
	}
	return st.Bavail * uint64(st.Bsize), nil
}

// lowDisk says whether the data directory's volume is short of space; an unset
// DiskFree or a failed read counts as fine.
func (r *JobRunner) lowDisk() (uint64, bool) {
	if r.DiskFree == nil {
		return 0, false
	}
	free, err := r.DiskFree()
	return free, err == nil && free < LowDiskBytes
}

// warnLowDisk tells the board, on the job's task, when the disk is still short of
// space. Job folders of unmerged work are never deleted to make room.
func (l *jobLoop) warnLowDisk() {
	if free, low := l.r.lowDisk(); low {
		l.note(fmt.Sprintf(
			"The runner's disk is low: %d GB free. Delete old job folders "+
				"or images on the Mac, or this job may fail.", free>>gigabyteShift))
	}
}

// MakeRoom cleans up before an image pull when the disk is low: the job folders
// of merged and finished tasks, then the old images (prune). It reports whether
// the disk is still low, and never deletes the folder of unmerged work.
func (r *JobRunner) MakeRoom(ctx context.Context, prune func(context.Context)) bool {
	if _, low := r.lowDisk(); !low {
		return false
	}
	r.Log.Warn("the disk is low; cleaning up before pulling the job image")
	r.CleanJobs(ctx)
	if prune != nil {
		prune(ctx)
	}
	free, low := r.lowDisk()
	if low {
		r.Log.Warn("the disk is still low after cleaning up", "free_gb", free>>gigabyteShift)
	}
	return low
}
