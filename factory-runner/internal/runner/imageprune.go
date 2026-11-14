package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// jobImageRepo is the local repository the versioned job images live in.
const (
	jobImageRepo = "factory-job:"
	imageDelete  = "delete"
)

// containerNames lists the names in a `container image list|ls --format json`
// output, taken from pick.
func containerNames(
	raw []byte, pick func(entry map[string]any) string,
) ([]string, error) {
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse container output: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if name := pick(entry); name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func nested(entry map[string]any, keys ...string) string {
	var cur any = entry
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	s, _ := cur.(string)
	return s
}

// jobImageTag returns the part from "factory-job:" on, or "" for any other
// image. The CLI reports local tags with a docker.io/library/ prefix.
func jobImageTag(name string) string {
	_, tag, ok := strings.Cut(name, "/"+jobImageRepo)
	if !ok {
		if !strings.HasPrefix(name, jobImageRepo) {
			return ""
		}
		tag = strings.TrimPrefix(name, jobImageRepo)
	}
	return jobImageRepo + tag
}

func listContainer(ctx context.Context, exec Execer, args ...string) ([]byte, error) {
	var out bytes.Buffer
	code, err := exec.Exec(
		ctx, append([]string{containerCLI}, args...), nil, &out,
	)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("container %s failed (exit %d)", args[0], code)
	}
	return out.Bytes(), nil
}

// PruneJobImages removes every local factory-job:* image except image and any
// a container (a running job's VM; several run at once) still references. It logs
// what it removed and returns nothing: a failed prune must not fail a job.
func PruneJobImages(
	ctx context.Context, exec Execer, image string, log *slog.Logger,
) {
	keep := jobImageTag(image)
	if keep == "" {
		return // an explicit image override or a dev build: not ours to prune
	}
	images, err := listContainer(ctx, exec, "image", "list", "--format", "json")
	if err != nil {
		log.Warn("job image prune: list images", "err", err)
		return
	}
	boxes, err := listContainer(ctx, exec, "ls", "--all", "--format", "json")
	if err != nil {
		log.Warn("job image prune: list containers", "err", err)
		return
	}
	refs, err := containerNames(boxes, func(m map[string]any) string {
		return nested(m, "configuration", "image", "reference")
	})
	if err != nil {
		log.Warn("job image prune: containers", "err", err)
		return
	}
	names, err := containerNames(images, func(m map[string]any) string {
		return nested(m, "configuration", "name")
	})
	if err != nil {
		log.Warn("job image prune: images", "err", err)
		return
	}
	inUse := map[string]bool{keep: true}
	for _, ref := range refs {
		inUse[jobImageTag(ref)] = true
	}
	for _, name := range names {
		tag := jobImageTag(name)
		if tag == "" || inUse[tag] {
			continue
		}
		code, derr := exec.Exec(
			ctx, []string{containerCLI, "image", imageDelete, name}, nil, io.Discard,
		)
		if derr != nil || code != 0 {
			log.Warn("job image prune: delete", "image", name, "exit", code, "err", derr)
			continue
		}
		log.Info("removed old job image", "image", name)
	}
}
