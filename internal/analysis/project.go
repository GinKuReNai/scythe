package analysis

// Project is language-neutral compiler evidence for a single analysis run.
type Project struct {
	DiscoveredFiles []string    `json:"discovered_files"`
	SourceRoots     []string    `json:"source_roots"`
	RootDir         string      `json:"root_dir"`
	TSConfigPath    string      `json:"tsconfig_path"`
	Framework       string      `json:"framework"`
	Files           int         `json:"files"`
	Complete        bool        `json:"complete"`
	Warnings        []string    `json:"warnings"`
	DynamicRisk     bool        `json:"dynamic_risk"`
	Symbols         []Symbol    `json:"-"`
	Edges           []Reference `json:"-"`
	Roots           []string    `json:"-"`
}

type Symbol struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	File            string `json:"file"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	Exported        bool   `json:"exported"`
	Public          bool   `json:"public"`
	FrameworkEntry  bool   `json:"framework_entry"`
	SideEffects     bool   `json:"side_effects"`
	DynamicRisk     bool   `json:"dynamic_risk"`
	Source          string `json:"source"`
	SourceTruncated bool   `json:"source_truncated"`
}

type Reference struct {
	From string `json:"from"`
	To   string `json:"to"`
	File string `json:"file"`
	Line int    `json:"line"`
}
