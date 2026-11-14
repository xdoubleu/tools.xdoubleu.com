package runner

import (
	"fmt"
	"strings"
)

// routinePreamble is the runner's own part of every routine's prompt, which the
// server can't change.
const routinePreamble = `You are running unattended in a sandbox with no network ` +
	`except the model. There is no GitHub: do not push or open a pull request. ` +
	`When you are done, write a short outcome to the file named by ` +
	`$FACTORY_OUTCOME_PATH: its first line says what the routine did, then what ` +
	`it found. If something outside your reach stops you, add one line ` +
	`"Blocked: <the problem>". Nobody can answer questions while you run.`

// routineBody is what a routine this Mac runs is told to do, by name; false for any
// other. The server is untrusted, so it can start a routine but never say what one
// does.
func routineBody(name string) (string, bool) {
	if name == "noop" {
		return "This is the no-op routine. Change nothing. Write the line " +
			`"No-op routine ran." as the first line of the outcome, then stop.`, true
	}
	return "", false
}

// routinePrompt is the prompt of a routine's job, or false for a routine this
// Mac doesn't run.
func routinePrompt(name string) (string, bool) {
	body, ok := routineBody(name)
	if !ok {
		return "", false
	}
	return routinePreamble + "\n\n# Routine: " + name + "\n\n" + body, true
}

// checkRoutine refuses a routine job the runner's own rules don't allow: a name
// outside its allowlist, a task id (a routine has none), or the branch the server
// names for it.
func checkRoutine(job *ClaimedJob) error {
	if _, ok := routinePrompt(job.Routine); !ok {
		return fmt.Errorf("%w: unknown routine %q", ErrRefused, job.Routine)
	}
	if job.TaskID != "" {
		return fmt.Errorf("%w: a routine has no task", ErrRefused)
	}
	if job.Branch != "agent/routine-"+job.Routine {
		return fmt.Errorf("%w: unexpected branch for routine %s", ErrRefused,
			job.Routine)
	}
	if strings.TrimSpace(job.ResumeSessionID) != "" {
		return fmt.Errorf("%w: a routine resumes no session", ErrRefused)
	}
	return nil
}
