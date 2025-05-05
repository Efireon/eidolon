package handlers

import (
	ers "eidolonVPN/internal/errors"
	"errors"
	"fmt"
	"io/fs"
)

func UtilsErrHandler(occfg string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ers.CallUtilsError("Utils config file not found, generated default", err)
	case errors.Is(err, fs.ErrPermission):
		return ers.CallUtilsError(fmt.Sprintf("Not enough permissions: %v", err), err)
	default:
		return ers.CallUtilsError(fmt.Sprintf("Unexpected error: %v", err), err)
	}
}
