package verapack

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-playground/validator/v10"
)

type VeraPackError struct {
	message     string
	application string
	task        string
}

func (v *VeraPackError) Error() string {
	// return fmt.Sprintf("%s (%s) ", v.application, v.task) + strings.Replace(v.message, "\n", fmt.Sprintf("\n%s (%s) ", v.application, v.task), strings.Count(v.message, "\n")-1)
	return v.message
	// return "an error occurred during package or scanning"
}

func NewVeraPackError(message, application, task string) *VeraPackError {
	return &VeraPackError{
		message:     message,
		application: application,
		task:        task,
	}
}

func renderErrors(errs ...error) string {
	var r string

	for k, err := range errs {
		var validateErrs validator.ValidationErrors
		var apiError veracode.Error
		if errors.As(err, &validateErrs) {
			for j, e := range validateErrs {
				var msg string
				switch e.Tag() {
				case "required":
					msg = fmt.Sprintf("config validation error at %s: field is required", e.Namespace())
				case "required_without":
					msg = fmt.Sprintf("config validation error at %s: either field '%s' or field '%s' is required", e.Namespace(), e.Field(), e.Param())
				case "oneof":
					msg = fmt.Sprintf("config validation error at %s: field value must be one of: [%v]", e.Namespace(), e.Param())
				case "gt":
					switch e.Kind() {
					case reflect.Ptr:
						fallthrough
					case reflect.Slice:
						msg = fmt.Sprintf("config validation error at %s: list field requires more than %s entries", e.Namespace(), e.Param())
					case reflect.Int:
						msg = fmt.Sprintf("config validation error at %s: number field needs to be bigger than %s", e.Namespace(), e.Param())
					default:
						fmt.Println("default")
					}
				case "file":
					msg = fmt.Sprintf("config validation error at %s: file '%s' does not exist", e.Namespace(), e.Value())
				case "dir":
					msg = fmt.Sprintf("config validation error at %s: directory '%s' does not exist", e.Namespace(), e.Value())
				case "file|dir":
					msg = fmt.Sprintf("config validation error at %s: file or directory '%s' does not exist", e.Namespace(), e.Value())
				case "min":
					msg = fmt.Sprintf("config validation error at %s: field must be greater or equal to: '%s'", e.Namespace(), e.Param())
				case "max":
					msg = fmt.Sprintf("config validation error at %s: field must be equal to or smaller than: '%s'", e.Namespace(), e.Param())
				default:
					msg = fmt.Sprintf("config validation error at %s: unspecified error with field, tag=%s,param=%s", e.Namespace(), e.Tag(), e.Param())
				}

				r += redForeground.Render("✗") + "  " + msg

				if j != len(validateErrs)-1 || (len(errs) > 1 && k != len(errs)-1) {
					// Can add a new line if not the last validation error
					// Or if there are more errors left after validation errors
					r += "\n"
				}
			}
		} else if errors.As(err, &apiError) {
			var msg string
			switch apiError.Code {
			case 401:
				msg = "an authentication error has occurred, please try manually regenerating your credentials by running: " + lightBlueForeground.Render("verapack credentials configure")
			case 403:
				msg = "the user does not have the correct permissions for this action, please contact your administrator to assist"
			default:
				msg = err.Error()
			}

			r += redForeground.Render("✗") + "  " + msg
			if k != len(errs)-1 {
				r += "\n"
			}

		} else {
			r += redForeground.Render("✗") + "  " + err.Error()
			if k != len(errs)-1 {
				r += "\n"
			}
		}
	}

	return lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1).
		PaddingBottom(1).
		MarginLeft(2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(darkGray).Render(headerStyle.Render("Errors")+"\n"+r) + "\n"
}
