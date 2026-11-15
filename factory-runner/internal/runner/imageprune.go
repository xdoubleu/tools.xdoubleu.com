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
	// pulledImageRepo names the digest-referenced copy `container image pull`
	// leaves next to the factory-job:<release> tag.
	pulledImageRepo = "/factory-job-image"
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

// pulledDigest returns the sha256 digest of a pulled factory-job-image name, ""
// for any other image. A name without a digest (an untagged one) gives "none".
func pulledDigest(name string) string {
	if !strings.Contains(name, pulledImageRepo) {
		return ""
	}
	if _, digest, ok := strings.Cut(name, "@"); ok {
		return digest
	}
	return "none"
}

// PruneJobImages removes every local factory-job:* image except image and any
// a container (a running job's VM; several run at once) still references, and
// the untagged factory-job-image pulls whose digest no kept image has. It logs
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
	var entries []map[string]any
	if err = json.Unmarshal(images, &entries); err != nil {
		log.Warn("job image prune: images", "err", err)
		return
	}
	inUse := map[string]bool{keep: true}
	for _, ref := range refs {
		inUse[jobImageTag(ref)] = true
	}
	keptDigest := keptDigests(entries, inUse)
	for _, entry := range entries {
		if !prunable(entry, inUse, keptDigest) {
			continue
		}
		name := nested(entry, "configuration", "name")
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

// keptDigests are the digests of the images that stay. A pulled copy shares its
// image with the kept tag, so it stays too.
func keptDigests(entries []map[string]any, inUse map[string]bool) map[string]bool {
	kept := map[string]bool{}
	for _, entry := range entries {
		name := nested(entry, "configuration", "name")
		if tag := jobImageTag(name); tag != "" && inUse[tag] {
			kept[nested(entry, "descriptor", "digest")] = true
		}
	}
	delete(kept, "")
	return kept
}

// prunable says whether an image is an old job image: a factory-job tag nothing
// uses, or a pulled factory-job-image copy no kept image shares.
func prunable(entry map[string]any, inUse, keptDigest map[string]bool) bool {
	name := nested(entry, "configuration", "name")
	if tag := jobImageTag(name); tag != "" {
		return !inUse[tag]
	}
	digest := pulledDigest(name)
	return digest != "" && !keptDigest[digest] &&
		!keptDigest[nested(entry, "descriptor", "digest")]
}
