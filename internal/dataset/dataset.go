package dataset

import (
	"bots/internal/config"
	"bots/internal/provider"
	"bots/internal/state"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Source struct {
	URI       string `json:"uri"`
	License   string `json:"license"`
	Revision  string `json:"revision"`
	Synthetic bool   `json:"synthetic"`
}
type Record struct {
	Line         int                `json:"-"`
	ID           string             `json:"id"`
	Task         string             `json:"task"`
	Group        string             `json:"group"`
	Split        string             `json:"split"`
	Source       Source             `json:"source"`
	Messages     []provider.Message `json:"messages"`
	OutputSchema map[string]string  `json:"output_schema,omitempty"`
}
type Rejection struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}
type Report struct {
	Accepted int         `json:"accepted"`
	Rejected []Rejection `json:"rejected"`
	SHA256   string      `json:"sha256"`
}

var secret = regexp.MustCompile(`(?i)(-----BEGIN .*PRIVATE KEY-----|(?:api[_-]?key|password|access[_-]?token)\s*[:=]\s*["']?[A-Za-z0-9_/-]{8,}|sk-[a-zA-Z0-9]{16,})`)

func fingerprint(s string) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(strings.ToLower(s)), " ")))
	return hex.EncodeToString(sum[:])
}
func input(r Record) string {
	var v []string
	for _, m := range r.Messages {
		if m.Role == "user" {
			v = append(v, m.Content)
		}
	}
	return strings.Join(v, "\n")
}
func validate(r Record) error {
	if !config.ValidID(r.ID) || r.Task == "" || r.Group == "" || r.Source.URI == "" || r.Source.License == "" || r.Source.Revision == "" {
		return errors.New("ID/task/group and source URI/license/revision required")
	}
	if r.Split != "train" && r.Split != "validation" && r.Split != "test" {
		return errors.New("split must be train, validation or test")
	}
	if len(r.Messages) < 2 || len(r.Messages) > 32 {
		return errors.New("messages must contain 2..32 text turns")
	}
	for i, m := range r.Messages {
		if secret.MatchString(m.Content) || state.Redact(m.Content) != m.Content {
			return errors.New("potential sensitive data; remove secrets before export")
		}
		if m.Content == "" || len(m.Content) > 65536 || len(m.ToolCalls) > 0 || m.ToolCallID != "" {
			return errors.New("nonempty bounded text messages required")
		}
		if m.Role == "system" {
			if i != 0 {
				return errors.New("system message must be first")
			}
			continue
		}
		expected := "user"
		start := 0
		if r.Messages[0].Role == "system" {
			start = 1
		}
		if (i-start)%2 == 1 {
			expected = "assistant"
		}
		if m.Role != expected {
			return errors.New("messages must alternate user/assistant")
		}
	}
	if r.Messages[len(r.Messages)-1].Role != "assistant" {
		return errors.New("last message must be assistant")
	}
	data, _ := json.Marshal(r)
	if secret.Match(data) || state.Redact(string(data)) != string(data) {
		return errors.New("potential sensitive data; remove secrets before export")
	}
	if len(r.OutputSchema) > 0 {
		var obj map[string]any
		if e := json.Unmarshal([]byte(r.Messages[len(r.Messages)-1].Content), &obj); e != nil || obj == nil {
			return errors.New("output must be a JSON object")
		}
		if len(obj) != len(r.OutputSchema) {
			return errors.New("output schema fields mismatch")
		}
		for key, kind := range r.OutputSchema {
			v, ok := obj[key]
			if !ok {
				return fmt.Errorf("output missing %s", key)
			}
			valid := false
			switch kind {
			case "string":
				_, valid = v.(string)
			case "number":
				_, valid = v.(float64)
			case "boolean":
				_, valid = v.(bool)
			case "array":
				_, valid = v.([]any)
			case "object":
				_, valid = v.(map[string]any)
			default:
				return fmt.Errorf("unsupported schema type %s", kind)
			}
			if !valid {
				return fmt.Errorf("output field %s requires %s", key, kind)
			}
		}
	}
	return nil
}
func Read(path string) ([]Record, Report, error) {
	report := Report{Rejected: []Rejection{}}
	f, e := os.Open(path)
	if e != nil {
		return nil, report, e
	}
	defer f.Close()
	h := sha256.New()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 4096), 1<<20)
	out := []Record{}
	line := 0
	for scan.Scan() {
		line++
		if line > 1000000 {
			return nil, report, errors.New("dataset exceeds one million lines")
		}
		raw := scan.Bytes()
		h.Write(raw)
		h.Write([]byte{'\n'})
		var r Record
		e := config.Decode(raw, &r)
		if e == nil {
			e = validate(r)
		}
		if e != nil {
			report.Rejected = append(report.Rejected, Rejection{line, e.Error()})
		} else {
			r.Line = line
			out = append(out, r)
		}
	}
	if e = scan.Err(); e != nil {
		return nil, report, e
	}
	report.SHA256 = hex.EncodeToString(h.Sum(nil))
	return out, report, nil
}
func Validate(path, evaluation string) ([]Record, Report, error) {
	records, report, e := Read(path)
	if e != nil {
		return nil, report, e
	}
	held := map[string]bool{}
	if evaluation == "" {
		return nil, report, errors.New("independent evaluation JSONL is required")
	}
	tests, testReport, e := Read(evaluation)
	if e != nil {
		return nil, report, e
	}
	if len(testReport.Rejected) > 0 || len(tests) == 0 {
		return nil, report, errors.New("evaluation records invalid or empty")
	}
	for _, r := range tests {
		held[fingerprint(input(r))] = true
	}
	ids := map[string]bool{}
	seen := map[string]bool{}
	groups := map[string]string{}
	out := []Record{}
	for _, r := range records {
		reason := ""
		key := fingerprint(input(r))
		switch {
		case ids[r.ID]:
			reason = "duplicate record ID"
		case seen[key]:
			reason = "duplicate or conflicting normalized input"
		case held[key]:
			reason = "input overlaps evaluation"
		case groups[r.Group] != "" && groups[r.Group] != r.Split:
			reason = "group leaks across splits"
		}
		ids[r.ID] = true
		seen[key] = true
		groups[r.Group] = r.Split
		if reason != "" {
			report.Rejected = append(report.Rejected, Rejection{r.Line, reason})
		} else {
			out = append(out, r)
		}
	}
	report.Accepted = len(out)
	if len(report.Rejected) > 0 {
		return nil, report, errors.New("dataset rejected; see line reasons")
	}
	if len(out) == 0 {
		return nil, report, errors.New("dataset empty")
	}
	return out, report, nil
}
