package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	"github.com/xdoubleu/essentia/v3/pkg/errortools"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type githubIssueRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type githubIssueResponse struct {
	HTMLURL string `json:"html_url"`
}

func (app *Application) bugReportHandler(w http.ResponseWriter, r *http.Request) {
	if app.config.GitHubToken == "" || app.config.GitHubRepo == "" {
		httptools.ErrorResponse(w, r,
			http.StatusServiceUnavailable,
			errors.New("bug reporting is not configured"),
		)
		return
	}

	var dto dtos.BugReportDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if strings.TrimSpace(dto.Title) == "" || strings.TrimSpace(dto.Description) == "" {
		httptools.FailedValidationResponse(w, r, map[string]string{
			"title":       "must not be empty",
			"description": "must not be empty",
		})
		return
	}

	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		httptools.UnauthorizedResponse(
			w, r,
			errortools.NewUnauthorizedError(errors.New("not signed in")),
		)
		return
	}

	body := buildIssueBody(
		dto.Description,
		dto.Page,
		r.Header.Get("User-Agent"),
		app.config.Release,
		app.config.Env,
		user.ID,
		app.requestBuffer.Get(user.ID),
	)

	issueURL, err := createGitHubIssue(
		r.Context(),
		app.config.GitHubToken,
		app.config.GitHubRepo,
		dto.Title,
		body,
	)
	if err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if err = httptools.WriteJSON(
		w,
		http.StatusOK,
		map[string]string{"url": issueURL},
		nil,
	); err != nil {
		httptools.HandleError(w, r, err)
	}
}

func buildIssueBody(
	description, page, userAgent, release, env, userID string,
	entries []logging.LogEntry,
) string {
	var sb strings.Builder
	sb.WriteString("## Description\n\n")
	sb.WriteString(description)
	sb.WriteString("\n\n## Context\n\n")
	sb.WriteString("**Page:** ")
	sb.WriteString(page)
	sb.WriteString("\n\n**Browser:** ")
	sb.WriteString(userAgent)
	sb.WriteString("\n\n**Release:** ")
	sb.WriteString(release)
	sb.WriteString("\n\n**Environment:** ")
	sb.WriteString(env)
	sb.WriteString("\n\n**User:** ")
	sb.WriteString(userID)
	sb.WriteString("\n\n## Recent logs\n\n")

	if len(entries) == 0 {
		sb.WriteString("_No log entries captured._\n")
	} else {
		sb.WriteString("| Time | Level | Message |\n")
		sb.WriteString("|---|---|---|\n")
		for _, e := range entries {
			fmt.Fprintf(&sb, "| %s | %s | %s |\n",
				e.Time.Format("2006-01-02 15:04:05"),
				e.Level,
				escapeMDTableCell(e.Message),
			)
		}
	}

	return sb.String()
}

func escapeMDTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func createGitHubIssue(
	ctx context.Context,
	token, repo, title, body string,
) (string, error) {
	payload, err := json.Marshal(githubIssueRequest{Title: title, Body: body})
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/issues", repo)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"GitHub API returned %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(respBody)),
		)
	}

	var ghResp githubIssueResponse
	if err = json.NewDecoder(resp.Body).Decode(&ghResp); err != nil {
		return "", err
	}

	return ghResp.HTMLURL, nil
}
