package workflow

import (
	"fmt"

	"github.com/talkersoft/hive-deck/internal/config"
)

// Assemble performs a depth-first expansion of a WorkflowDef into an ordered
// slice of fragment refs (@ prefix intact). Errors on missing job or cycle.
func Assemble(def *config.WorkflowDef, jobs map[string]config.JobDef, tokens map[string]string) ([]string, error) {
	if err := config.DetectCycles(jobs); err != nil {
		return nil, err
	}
	var out []string
	if err := emitBody(&out, config.JobBody{Init: def.Tasks.Init, Jobs: def.Tasks.Jobs, Post: def.Tasks.Post}, jobs); err != nil {
		return nil, err
	}
	return out, nil
}

func emitBody(out *[]string, body config.JobBody, jobs map[string]config.JobDef) error {
	*out = append(*out, body.Init...)
	for _, jobName := range body.Jobs {
		def, ok := jobs[jobName]
		if !ok {
			return fmt.Errorf("job %q not found", jobName)
		}
		if len(def.Fragments) > 0 {
			*out = append(*out, def.Fragments...)
		} else {
			if err := emitBody(out, config.JobBody{Init: def.Init, Jobs: def.Jobs, Post: def.Post}, jobs); err != nil {
				return err
			}
		}
	}
	*out = append(*out, body.Post...)
	return nil
}
