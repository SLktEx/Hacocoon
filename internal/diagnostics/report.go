// Package diagnostics defines the read-only Host diagnostic projection shared
// by provider adapters and controller clients. It grants no repair authority.
package diagnostics

import "fmt"

const (
	OK      = "ok"
	Failed  = "failed"
	Skipped = "skipped"
)

func CheckNames() [5]string {
	return [5]string{"runtime", "storage", "trusted_host", "trusted_network", "trusted_connectivity"}
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Action  string `json:"action,omitempty"`
}

type Report struct {
	Checks []Check `json:"checks"`
}

// Validate prevents a partial/malformed peer response from becoming a clean
// bill of health. Summaries are bounded printable text, never provider output.
func (r Report) Validate() error {
	names := CheckNames()
	if len(r.Checks) != len(names) {
		return fmt.Errorf("incomplete Host diagnostics")
	}
	for i, check := range r.Checks {
		if check.Name != names[i] || (check.Status != OK && check.Status != Failed && check.Status != Skipped) || len(check.Summary) == 0 || len(check.Summary) > 256 {
			return fmt.Errorf("invalid Host diagnostic check")
		}
		if len(check.Action) > 256 || (check.Status != OK && len(check.Action) == 0) || (check.Status == OK && check.Action != "") {
			return fmt.Errorf("invalid Host diagnostic action")
		}
		for _, c := range check.Summary + check.Action {
			if c < 32 || c > 126 {
				return fmt.Errorf("invalid Host diagnostic summary")
			}
		}
	}
	return nil
}

func (r Report) Healthy() bool {
	if r.Validate() != nil {
		return false
	}
	for _, check := range r.Checks {
		if check.Status != OK {
			return false
		}
	}
	return true
}
