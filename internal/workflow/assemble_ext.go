package workflow

import "fmt"

// AssembleFromExtension builds an AssembledWorkflow from a BuiltinBase + optional
// WorkflowExtension. User phases slot between the builtin Init (task 0000) and
// the builtin Post (task FINAL).
//
// Rules:
//   - ext.ExtraInit fragments are appended to base.Init.Fragments in Task 0000.
//   - ext.Phases each become one numbered task between init and post.
//   - ext.ExtraPost fragments are appended to base.Post.Fragments in Task FINAL.
//   - If ext is non-nil and has no phases AND no extra_init/extra_post, error.
func AssembleFromExtension(base BuiltinBase, ext *WorkflowExtension, tokens map[string]string) (*AssembledWorkflow, error) {
	if ext != nil && len(ext.Phases) == 0 && len(ext.Init) == 0 && len(ext.Post) == 0 && len(ext.Mcps) == 0 && len(ext.Build) == 0 && len(ext.Deploy) == 0 {
		return nil, fmt.Errorf("workflow extension has no phases, init, post, mcps, build, or deploy defined")
	}

	extTokens := map[string]string(nil)
	if ext != nil {
		extTokens = ext.Tokens
	}
	merged := mergeTokens(tokens, extTokens)

	aw := &AssembledWorkflow{Type: base.Type, Tokens: merged}

	mcpsFrags := append([]string(nil), base.Mcps.Fragments...)
	if ext != nil {
		mcpsFrags = append(mcpsFrags, ext.Mcps...)
	}
	if len(mcpsFrags) > 0 {
		aw.Tasks = append(aw.Tasks, AssembledTask{
			Number:    "mcps",
			Title:     "Available MCPs",
			Source:    "builtin",
			Fragments: mcpsFrags,
		})
	}

	if ext != nil && len(ext.Build) > 0 {
		aw.Tasks = append(aw.Tasks, AssembledTask{
			Number:    "build",
			Title:     "Build requirements",
			Source:    "builtin",
			Fragments: append([]string(nil), ext.Build...),
		})
	}

	if ext != nil && len(ext.Deploy) > 0 {
		aw.Tasks = append(aw.Tasks, AssembledTask{
			Number:    "deploy",
			Title:     "Deploy",
			Source:    "builtin",
			Fragments: append([]string(nil), ext.Deploy...),
		})
	}

	initFrags := append([]string(nil), base.Init.Fragments...)
	if ext != nil {
		initFrags = append(initFrags, ext.Init...)
	}
	aw.Tasks = append(aw.Tasks, AssembledTask{
		Number:    "0000",
		Title:     "Initialize",
		Source:    "builtin",
		Fragments: initFrags,
	})

	if ext != nil {
		for i, p := range ext.Phases {
			num := fmt.Sprintf("%04d", i+1)
			aw.Tasks = append(aw.Tasks, AssembledTask{
				Number:    num,
				Title:     p.Title,
				Source:    "user",
				Fragments: p.Jobs,
			})
		}
	}

	postFrags := append([]string(nil), base.Post.Fragments...)
	if ext != nil {
		postFrags = append(postFrags, ext.Post...)
	}
	aw.Tasks = append(aw.Tasks, AssembledTask{
		Number:    "FINAL",
		Title:     "Results and ship",
		Source:    "builtin",
		Fragments: postFrags,
	})

	return aw, nil
}

// AssembleFromPlanExtension builds an AssembledWorkflow for the "plan" type.
// Base fragments come first, then extension fragments are appended.
func AssembleFromPlanExtension(base BuiltinBase, ext *PlanExtension, tokens map[string]string) (*AssembledWorkflow, error) {
	extTokens := map[string]string(nil)
	if ext != nil {
		extTokens = ext.Tokens
	}
	merged := mergeTokens(tokens, extTokens)

	var frags []string
	frags = append(frags, base.Fragments...)
	if ext != nil {
		frags = append(frags, ext.Fragments...)
	}

	aw := &AssembledWorkflow{
		Type:   base.Type,
		Tokens: merged,
		Tasks: []AssembledTask{
			{Number: "plan", Source: "builtin", Fragments: frags},
		},
	}
	return aw, nil
}

func mergeTokens(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
