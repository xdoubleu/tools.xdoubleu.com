package runner

import (
	"context"
	"fmt"
)

// maxFilesPercent is the share of the system's open-file limit above which
// claims hold: the host keeps a descriptor for every file a job VM's guest looks
// up on its shared mounts, and at the limit every process on the Mac fails.
const maxFilesPercent = 70

// FilePressurePreflight fails while the files open on the system pass
// maxFilesPercent of its limit. read reports how many are open and the limit; a
// read that fails or knows no limit passes, to let a job tell.
func FilePressurePreflight(
	read func() (open, limit uint32, err error),
) func(context.Context) error {
	return func(context.Context) error {
		open, limit, err := read()
		if err != nil || limit == 0 {
			return nil //nolint:nilerr // an unknown count holds nothing
		}
		if uint64(open)*100 > uint64(limit)*maxFilesPercent {
			return fmt.Errorf("%d of %d system files open (kern.maxfiles), "+
				"above %d%%; claims resume below it",
				open, limit, maxFilesPercent)
		}
		return nil
	}
}

// Preflights runs each check in turn and fails on the first that fails.
func Preflights(checks ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		for _, check := range checks {
			if err := check(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}
