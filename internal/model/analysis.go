package model

type Recommendation struct {
	Path          string   `json:"path"`
	Category      string   `json:"category"`
	SizeBytes     int64    `json:"size_bytes"`
	Risk          string   `json:"risk"`
	Confidence    float64  `json:"confidence"`
	Reason        string   `json:"reason"`
	Evidence      []string `json:"evidence"`
	VerifyBefore  []string `json:"verify_before_delete"`
	RegenerableBy string   `json:"regenerable_by,omitempty"`
}

type Coverage struct {
	EntriesScanned   int  `json:"entries_scanned"`
	EntriesInspected int  `json:"entries_inspected"`
	Rounds           int  `json:"rounds"`
	ToolCalls        int  `json:"tool_calls"`
	Complete         bool `json:"complete"`
}

type AnalysisResult struct {
	Summary         string           `json:"summary"`
	Recommendations []Recommendation `json:"recommendations"`
	Keep            []Recommendation `json:"keep"`
	Unknown         []Recommendation `json:"unknown"`
	Coverage        Coverage         `json:"coverage"`
	Warnings        []string         `json:"warnings"`
}
