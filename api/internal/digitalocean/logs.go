package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"tools.xdoubleu.com/internal/oauthconn"
)

// logContentCap bounds how much of a single component/type's log text is
// kept, so one huge log can't blow up the Connect response or the MCP tool
// payload handed to a Claude session.
const logContentCap = 200 * 1024 // 200 KB

// errBodyCap bounds how much of an error response body is read for the
// error message when a log-fetch request fails.
const errBodyCap = 4096

// logTypes is every log phase DeploymentLogs fetches, in display order:
// build/deploy phases first, then the component's runtime logs.
//
//nolint:gochecknoglobals // read-only constant list, mirrors a Go array literal
var logTypes = []LogType{
	LogTypeBuild, LogTypeDeploy, LogTypeRun, LogTypeRunRestarted,
}

func (c *client) DeploymentLogs(
	ctx context.Context, deploymentID string,
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

	logs := make([]ComponentLog, 0, len(components)*len(logTypes))
	for _, component := range components {
		for _, logType := range logTypes {
			log, ok, fetchErr := c.componentLog(
				ctx, token, appID, deploymentID, component, logType,
			)
			if fetchErr != nil {
				return nil, fetchErr
			}
			if ok {
				logs = append(logs, log)
			}
		}
	}
	return logs, nil
}

// serviceComponents reads the deployment's service component names off its
// spec, so DeploymentLogs knows which component_name values to query.
func (c *client) serviceComponents(
	ctx context.Context, token, appID, deploymentID string,
) ([]string, error) {
	endpoint := fmt.Sprintf(
		"%s/v2/apps/%s/deployments/%s", baseURL, appID, deploymentID,
	)

	var wire deploymentDetailWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(wire.Deployment.Spec.Services))
	for _, svc := range wire.Deployment.Spec.Services {
		names = append(names, svc.Name)
	}
	return names, nil
}

// componentLog fetches one component's log text for one LogType. ok is false
// when the deployment never reached that phase for that component (DO
// returns no historic URLs), which is a valid state, not an error.
func (c *client) componentLog(
	ctx context.Context, token, appID, deploymentID, component string, logType LogType,
) (ComponentLog, bool, error) {
	query := url.Values{}
	query.Set("component_name", component)
	query.Set("type", string(logType))
	endpoint := fmt.Sprintf(
		"%s/v2/apps/%s/deployments/%s/logs?%s",
		baseURL, appID, deploymentID, query.Encode(),
	)

	var wire deployLogsWire
	if err := c.get(ctx, endpoint, token, &wire); err != nil {
		return ComponentLog{}, false, err
	}
	if len(wire.HistoricURLs) == 0 {
		return ComponentLog{}, false, nil //nolint:exhaustruct // no log yet, not an error
	}

	content, truncated, err := c.fetchLogContent(ctx, wire.HistoricURLs)
	if err != nil {
		return ComponentLog{}, false, err
	}

	return ComponentLog{
		Component: component,
		Type:      logType,
		Content:   content,
		Truncated: truncated,
	}, true, nil
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
