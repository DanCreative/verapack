package verapack

import (
	"fmt"
	"strings"
)

type VeraPackError struct {
	message     string
	application string
	task        string
}

func (v *VeraPackError) Error() string {
	return fmt.Sprintf("%s (%s) ", v.application, v.task) + strings.Replace(v.message, "\n", fmt.Sprintf("\n%s (%s) ", v.application, v.task), strings.Count(v.message, "\n")-1)
	// return "an error occurred during package or scanning"
}

func NewVeraPackError(message, application, task string) *VeraPackError {
	return &VeraPackError{
		message:     message,
		application: application,
		task:        task,
	}
}
