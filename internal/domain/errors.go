package domain

import "fmt"

type ErrBadURL struct {
	rawURL string
}

func NewErrBadURL(rawURL string) error {
	return &ErrBadURL{
		rawURL: rawURL,
	}
}

func (e *ErrBadURL) Error() string {
	return fmt.Sprintf("given url has invalid format: %v", e.rawURL)
}
