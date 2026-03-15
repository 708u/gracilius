package comment

// DiffContext identifies a diff comparison context for comment grouping.
type DiffContext struct {
	Kind      string `json:"kind"`                // "working", "branch", "review"
	Base      string `json:"base,omitempty"`      // base branch name (for "branch")
	SessionID string `json:"sessionId,omitempty"` // session UUID (for "review")
}

// DiffCommentsFile is the top-level structure for a diff comments JSON file.
type DiffCommentsFile struct {
	RootDir  string      `json:"rootDir"`
	Version  int         `json:"version"`
	Context  DiffContext `json:"context"`
	Comments []Entry     `json:"comments"`
}

// Key returns the file name stem for this context.
func (c DiffContext) Key() string {
	switch c.Kind {
	case "branch":
		if c.Base != "" {
			return "branch-" + c.Base
		}
		return "branch-main"
	case "review":
		if c.SessionID != "" {
			return "review-" + c.SessionID
		}
		return "review"
	default:
		return "working"
	}
}
