package comment

// DiffScope identifies a diff comparison scope for comment grouping.
// Zero value represents a file comment (no diff scope).
type DiffScope struct {
	Kind      string `json:"kind"`                // "working", "branch", "review"
	Base      string `json:"base,omitempty"`      // base branch name (for "branch")
	SessionID string `json:"sessionId,omitempty"` // session UUID (for "review")
}
