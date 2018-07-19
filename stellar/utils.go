package stellar

import (
	"fmt"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/errors"
	"sort"
)

// validateFields verifies that no bad arguments were given to the request.
func validateFields(req *logical.Request, data *framework.FieldData) error {
	var unknownFields []string
	for k := range req.Data {
		if _, ok := data.Schema[k]; !ok {
			unknownFields = append(unknownFields, k)
		}
	}

	if len(unknownFields) > 0 {
		// Sort since this is a human error
		sort.Strings(unknownFields)

		return fmt.Errorf("unknown fields: %q", unknownFields)
	}

	return nil
}

// errMissingField returns a logical response error that prints a consistent
// error message for when a required field is missing.
func errMissingField(field string) *logical.Response {
	return logical.ErrorResponse(fmt.Sprintf("Missing required field '%s'", field))
}

// ErrorString parses the horizon error out of err.
func ErrorString(err error, showStackTrace ...bool) string {
	var errorString string
	herr, isHorizonError := errors.Cause(err).(*horizon.Error)

	if isHorizonError {
		errorString += fmt.Sprintf("%v: %v", herr.Problem.Status, herr.Problem.Title)

		resultCodes, err := herr.ResultCodes()
		if err == nil {
			errorString += fmt.Sprintf(" (%v)", resultCodes)
		}
	} else {
		errorString = fmt.Sprintf("%v", err)
	}

	if len(showStackTrace) > 0 {
		if isHorizonError {
			errorString += fmt.Sprintf("\nDetail: %s\nType: %s\n", herr.Problem.Detail, herr.Problem.Type)
		}
		errorString += fmt.Sprintf("\nStack trace:\n%+v\n", err)
	}

	return errorString
}
