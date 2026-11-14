package rules

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

type Rule struct {
	ID             string   `yaml:"id"`
	Title          string   `yaml:"title"`
	Severity       Severity `yaml:"severity"`
	Collector      string   `yaml:"collector"`
	ConditionType  string   `yaml:"condition_type"`
	Threshold      float64  `yaml:"threshold"`
	Recommendation string   `yaml:"recommendation"`
	Explanation    string   `yaml:"explanation"`
	DocURL         string   `yaml:"doc_url"`
}

type Finding struct {
	Rule           Rule
	Evidence       string
	Recommendation string
}
