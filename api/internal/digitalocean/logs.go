package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/internal/oauthconn"
)

// collectLogsConcurrency bounds how many component/type log fetches run at
// once, so a deployment with many components can't open dozens of
// simultaneous connections (including live-socket reads) to DigitalOcean.
const collectLogsConcurrency = 4

// logContentCap bounds how much of a single component/type's log text is
// kept, so one huge log can't blow up the Connect response or the MCP tool
// payload handed to a Claude session.
const logContentCap = 200 * 1024 // 200 KB

// errBodyCap bounds how much of an error response body is read for the
// error message when a log-fetch request fails.
const errBodyCap = 4096

// tail_lines bounds how much backlog DigitalOcean replays over a component's
// live log socket. It has to be set explicitly: without it the socket only
// pushes lines produced *after* it connects, so a quiet component returns
// nothing at all.
const (
	defaultTailLines = 1000
	maxTailLines     = 10000
)

// logTypes is every log phase fetched for the requested deployment, in
// display order: build/deploy phases first, then the component's runtime logs.
//
//nolint:gochecknoglobals // read-only constant list, mirrors a Go array literal
var logTypes = []LogType{
	LogTypeBuild, LogTypeDeploy, LogTypeRun, LogTypeRunRestarted,
}

// runtimeLogTypes is what is additionally fetched from the app's active
// deployment when it isn't the deployment that was asked for.
//
//nolint:gochecknoglobals // read-only constant list, mirrors a Go array literal
var runtimeLogTypes = []LogType{LogTypeRun, LogTypeRunRestarted}

// logScope is one batch of component logs: the /logs endpoint they are read
// from and the deployment ID the resulting blocks are tagged with. The
// app-scoped endpoint carries no deployment ID of its own — DigitalOcean
// resolves it against whichever deployment is currently serving traffic.
type logScope struct {
	endpoint     string
	deploymentID string
	components   []string
}

func (c *client) DeploymentLogs(
	ctx context.Context, deploymentID string, tailLines int,
) ([]ComponentLog, error) {
	appID, err := c.resolveAppID(ctx)
	if err != nil {
		return nil, err
	}

	token, err := c.tokenFn(ctx)
	if errors.Is(err, oauthconn.ErrNotConnected) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	components, err := c.serviceComponents(ctx, token, appID, deploymentID)
	if err != nil {
		return nil, err
	}

	tail := clampTailLines(tailLines)
	logs, err := c.collectLogs(ctx, token, logScope{
		endpoint: fmt.Sprintf(
			"%s/v2/apps/%s/deployments/%s/logs", baseURL, appID, deploymentID,
		),
		deploymentID: deploymentID,
		components:   components,
	}, logTypes, tail)
	if err != nil {
		return nil, err
	}

	runtime := c.activeRuntimeLogs(ctx, token, appID, deploymentID, tail)
	return append(logs, runtime...), nil
}

// activeRuntimeLogs fetches the runtime logs of the deployment that is
// actually serving traffic, when that isn't the deployment being asked about
// — the case where the latest deploy failed and the previous one kept
// running, which is exactly when the app's own output matters most. It
// degrades to nothing on error: the requested deployment's logs are already
// collected and are worth returning on their own.
func (c *client) activeRuntimeLogs(
	ctx context.Context, token, appID, deploymentID string, tail int,
) []ComponentLog {
	active, err := c.activeDeployment(ctx, token, appID)
	if err != nil {
		c.logger.WarnContext(ctx, "digitalocean active deployment unavailable",
			slog.Any("error", err))
		return nil
	}
	if active.id == "" || active.id == deploymentID {
		return nil
	}

	logs, err := c.collectLogs(ctx, token, logScope{
		endpoint:     fmt.Sprintf("%s/v2/apps/%s/logs", baseURL, appID),
		deploymentID: active.id,
		components:   active.components,
	}, runtimeLogTypes, tail)
	if err != nil {
		c.logger.WarnContext(ctx, "digitalocean runtime logs unavailable",
			slog.String("deployment_id", active.id), slog.Any("error", err))
		return nil
	}
	return logs
}

// collectLogs fetches every component/type pair of one scope concurrently
// (bounded by collectLogsConcurrency), skipping the pairs that produced no
// text at all. Results are returned in the same component/type order they
// were requested in, regardless of fetch completion order.
func (c *client) collectLogs(
	ctx context.Context, token string, scope logScope, types []LogType, tail int,
) ([]ComponentLog, error) {
	type slot struct {
		log ComponentLog
		ok  bool
	}

	slots := make([]slot, 0, len(scope.components)*len(types))
	for range scope.components {
		for range types {
			slots = append(slots, slot{}) //nolint:exhaustruct // filled below
		}
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(collectLogsConcurrency)

	i := 0
	for _, component := range scope.components {
		for _, logType := range types {
			idx, comp, lt := i, component, logType
			i++
			group.Go(func() error {
				log, ok, err := c.componentLog(groupCtx, token, scope, comp, lt, tail)
				if err != nil {
					return err
				}
				slots[idx] = slot{log: log, ok: ok}
				return nil
			})
		}
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	logs := make([]ComponentLog, 0, len(slots))
	for _, s := range slots {
		if s.ok {
			logs = append(logs, s.log)
		}
	}
	return logs, nil
}

// serviceComponents reads the deployment's service component names off its
// top-level services list, so DeploymentLogs knows which component_name values
// to query. Only services carry BUILD/DEPLOY logs; workers/jobs are omitted.
func (c *client) serviceComponents(
	ctx context.Context, token, appID, deploymentID string,
) ([]string, error) {
	endpoint := fmt.Sprintf(
		"%s/v2/apps/%s/deployments/%s", baseURL, appID, deploymentID,
	)

	var wire deploymentDetailWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return nil, fmt.Errorf("service components: %w", err)
	}

	names := make([]string, 0, len(wire.Deployment.Services))
	for _, svc := range wire.Deployment.Services {
		names = append(names, svc.Name)
	}
	return names, nil
}

// activeDeploymentInfo identifies the deployment currently serving traffic
// and the service components it runs.
type activeDeploymentInfo struct {
	id         string
	components []string
}

// activeDeployment reads the app's active deployment off the app-detail
// endpoint. Component names come from the deployment's top-level services
// list for the same reason serviceComponents uses it — spec is omitempty.
func (c *client) activeDeployment(
	ctx context.Context, token, appID string,
) (activeDeploymentInfo, error) {
	endpoint := fmt.Sprintf("%s/v2/apps/%s", baseURL, appID)

	var wire appDetailWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return activeDeploymentInfo{}, err
	}

	active := wire.App.ActiveDeployment
	names := make([]string, 0, len(active.Services))
	for _, svc := range active.Services {
		names = append(names, svc.Name)
	}
	return activeDeploymentInfo{id: active.ID, components: names}, nil
}

// componentLog fetches one component's log text for one LogType: the archived
// chunks first, then whatever the live socket replays for a component that is
// still running. ok is false when neither source produced anything, which
// means the deployment never reached that phase for that component — a valid
// state, not an error.
func (c *client) componentLog(
	ctx context.Context,
	token string,
	scope logScope,
	component string,
	logType LogType,
	tail int,
) (ComponentLog, bool, error) {
	query := url.Values{}
	query.Set("component_name", component)
	query.Set("type", string(logType))
	query.Set("follow", "false")
	query.Set("tail_lines", strconv.Itoa(tail))

	var wire deployLogsWire
	err := c.get(ctx, scope.endpoint+"?"+query.Encode(), token, &wire)
	if err != nil {
		if isSkippedLogTaskErr(err) {
			//nolint:exhaustruct // phase never ran, not an error
			return ComponentLog{}, false, nil
		}
		return ComponentLog{}, false, fmt.Errorf(
			"component %s type %s: %w", component, logType, err,
		)
	}

	content, truncated, err := c.fetchLogContent(ctx, wire.HistoricURLs)
	if err != nil {
		return ComponentLog{}, false, err
	}
	if !truncated && wire.LiveURL != "" {
		live, liveTruncated := c.fetchLiveContent(
			ctx, wire.LiveURL, logContentCap-len(content),
		)
		content += live
		truncated = liveTruncated
	}
	if content == "" {
		return ComponentLog{}, false, nil //nolint:exhaustruct // no log yet
	}

	return ComponentLog{
		Component:    component,
		Type:         logType,
		DeploymentID: scope.deploymentID,
		Content:      content,
		Truncated:    truncated,
	}, true, nil
}

// isSkippedLogTaskErr reports whether err is DigitalOcean's 400 for a
// component/type pair whose phase never ran (e.g. a deploy that reused a
// prior build) — a normal, expected state rather than a real failure.
func isSkippedLogTaskErr(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && apiErr.status == http.StatusBadRequest &&
		strings.Contains(apiErr.body, "log task status skipped")
}

// clampTailLines keeps a caller-supplied backlog size inside sane bounds; 0
// (the unset default) picks defaultTailLines.
func clampTailLines(tailLines int) int {
	if tailLines <= 0 {
		return defaultTailLines
	}
	if tailLines > maxTailLines {
		return maxTailLines
	}
	return tailLines
}

// fetchLogContent concatenates every historic URL's content (oldest first,
// as DO returns them) up to logContentCap total bytes. The URLs are
// pre-signed — no Authorization header is sent.
func (c *client) fetchLogContent(
	ctx context.Context, urls []string,
) (string, bool, error) {
	var content []byte
	for _, u := range urls {
		if len(content) >= logContentCap {
			return string(content), true, nil
		}

		chunk, err := c.fetchURL(ctx, u, logContentCap-len(content))
		if err != nil {
			return "", false, err
		}
		content = append(content, chunk...)
	}
	return string(content), false, nil
}

func (c *client) fetchURL(ctx context.Context, u string, limit int) ([]byte, error) {
	var body []byte
	err := c.doWithRetry(ctx, func() (bool, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if reqErr != nil {
			return false, reqErr
		}

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, errBodyCap))
			return true, fmt.Errorf(
				"digitalocean log fetch returned %d: %s", resp.StatusCode, string(raw),
			)
		}
		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, errBodyCap))
			return false, fmt.Errorf(
				"digitalocean log fetch returned %d: %s", resp.StatusCode, string(raw),
			)
		}

		read, readErr := io.ReadAll(io.LimitReader(resp.Body, int64(limit)))
		if readErr != nil {
			return false, readErr
		}
		body = read
		return false, nil
	})
	return body, err
}
