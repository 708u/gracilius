package comment

// DiffScope identifies a diff comparison scope for comment grouping.
type DiffScope struct {
	Kind      string `json:"kind"`                // "working", "branch", "review"
	Base      string `json:"base,omitempty"`      // base branch name (for "branch")
	SessionID string `json:"sessionId,omitempty"` // session UUID (for "review")
}

// DiffCommentsFile is the top-level structure for a diff comments JSON file.
type DiffCommentsFile struct {
	RootDir  string    `json:"rootDir"`
	Version  int       `json:"version"`
	Scope    DiffScope `json:"scope"`
	Comments []Entry   `json:"comments"`
}

// Key returns the file name stem for this scope.
func (c DiffScope) Key() string {
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
