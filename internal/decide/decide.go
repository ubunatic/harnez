// Package decide runs typed decisions (noul, choice, score) against a
// decision model such as TypeSafe Jev (issue 633). Backends are declared in
// spec/decide.yaml; each speaks one wire protocol, so a local model (Laya,
// lmcoder, ...) plugs in as a spec entry, or as a new protocol behind the
// Backend interface.
package decide

import (
	"context"
	"fmt"
	"sort"
)

// Question types understood by every backend.
const (
	TypeNoul   = "noul"
	TypeChoice = "choice"
	TypeScore  = "score"
)

// Request is one decision call: a state and named questions about it.
type Request struct {
	Model     string              `json:"model,omitempty" yaml:"model,omitempty"`
	State     any                 `json:"state" yaml:"state"`
	Questions map[string]Question `json:"questions" yaml:"questions"`
}

// Question asks one typed question. Criteria is a map for noul ("true",
// "false") and choice (option -> description), and a list of level
// descriptions, lowest first, for score.
type Question struct {
	Type         string `json:"type" yaml:"type"`
	Instructions string `json:"instructions" yaml:"instructions"`
	Criteria     any    `json:"criteria,omitempty" yaml:"criteria,omitempty"`
}

// Answer holds one typed answer; fields not used by its Type are zero.
type Answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

// Usage reports token counts when the backend provides them.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response is the result of one Request.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Backend answers decision requests. Implementations must be safe for
// concurrent use.
type Backend interface {
	Decide(ctx context.Context, req *Request) (*Response, error)
}

// Validate checks the request shape before it is sent.
func (r *Request) Validate() error {
	if len(r.Questions) == 0 {
		return fmt.Errorf("request has no questions")
	}
	for _, id := range r.QuestionIDs() {
		q := r.Questions[id]
		switch q.Type {
		case TypeNoul, TypeChoice, TypeScore:
		default:
			return fmt.Errorf("question %q: type %q is not noul, choice or score", id, q.Type)
		}
		if q.Instructions == "" {
			return fmt.Errorf("question %q: instructions are empty", id)
		}
	}
	return nil
}

// QuestionIDs returns the question ids in sorted order.
func (r *Request) QuestionIDs() []string {
	ids := make([]string, 0, len(r.Questions))
	for id := range r.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Value is the answer as one plain string: the noul probability, the chosen
// option, or the score level.
func (a Answer) Value() string {
	switch a.Type {
	case TypeNoul:
		if a.Noul != nil {
			return fmt.Sprintf("%.2f", *a.Noul)
		}
	case TypeChoice:
		return a.Choice
	case TypeScore:
		if a.Score != nil {
			return fmt.Sprintf("%g", *a.Score)
		}
	}
	return ""
}

// Level is the number a threshold is compared with: the noul probability,
// the chosen option's probability, or the score level.
func (a Answer) Level() (float64, bool) {
	switch a.Type {
	case TypeNoul:
		if a.Noul != nil {
			return *a.Noul, true
		}
	case TypeChoice:
		p, ok := a.Probabilities[a.Choice]
		return p, ok
	case TypeScore:
		if a.Score != nil {
			return *a.Score, true
		}
	}
	return 0, false
}
