package errors

import stderrors "errors"

func New(msg string) error {
	return stderrors.New(msg)
}

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return stderrors.New(msg + ": " + err.Error())
}

func Is(err error, target error) bool {
	return stderrors.Is(err, target)
}
