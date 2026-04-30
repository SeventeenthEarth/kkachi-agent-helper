package project

import "fmt"

// Problem is a deterministic, machine-mappable project safety error.
type Problem struct {
	Code     string
	Message  string
	Hint     string
	Path     string
	Field    string
	Expected string
	Actual   string
}

func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	if p.Path == "" {
		return p.Message
	}
	return fmt.Sprintf("%s: %s", p.Message, p.Path)
}

func problem(code string, message string, hint string) *Problem {
	return &Problem{
		Code:    code,
		Message: message,
		Hint:    hint,
	}
}
