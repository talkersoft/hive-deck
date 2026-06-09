package workflow

// WorkflowExtension is loaded from ~/.hv/workflows/workflow/<name>.yaml.
// It fills the slot between BuiltinBase.Init and BuiltinBase.Post.
// Init fragments are appended to the built-in init section (Task 0000).
// Post fragments are appended to the built-in post section (Task FINAL).
// Mcps fragments are appended to the built-in mcps preamble (rendered before Task 0000).
// Build fragments render after mcps — per-task build requirements for this workflow.
// Deploy fragments render after build — how to deploy after a successful ship.
type WorkflowExtension struct {
	Tokens map[string]string `yaml:"tokens"`
	Mcps   []string          `yaml:"mcps"`
	Build  []string          `yaml:"build"`
	Deploy []string          `yaml:"deploy"`
	Init   []string          `yaml:"init"`
	Phases []Phase           `yaml:"phases"`
	Post   []string          `yaml:"post"`
}

// Phase maps to one TASK file. Jobs are sequential steps within the task.
type Phase struct {
	Title string   `yaml:"title"`
	Jobs  []string `yaml:"jobs"`
}

// PlanExtension appends fragments after the base plan fragments.
type PlanExtension struct {
	Tokens    map[string]string `yaml:"tokens"`
	Fragments []string          `yaml:"fragments"`
}

// BuiltinBase is the embedded base loaded from internal/builtin/<type>/_base.yaml.
type BuiltinBase struct {
	Type      string      `yaml:"type"`
	Mcps      BaseSection `yaml:"mcps"`
	Init      BaseSection `yaml:"init"`
	Post      BaseSection `yaml:"post"`
	Fragments []string    `yaml:"fragments"` // for plan type
}

type BaseSection struct {
	Fragments []string `yaml:"fragments"`
}

// AssembledWorkflow is the merged result passed to the renderer.
type AssembledWorkflow struct {
	Type   string
	Tokens map[string]string
	Tasks  []AssembledTask
}

// AssembledTask is one task in the assembled workflow.
type AssembledTask struct {
	Number    string   // "0000", "0001..N", "FINAL"
	Title     string
	Source    string   // "builtin" | "user"
	Fragments []string
}
