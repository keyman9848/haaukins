package store

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	tagRawRegexp = `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
	tagRegex     = regexp.MustCompile(tagRawRegexp)
	TagEmptyErr  = errors.New("Tag cannot be empty")
)

type InvalidTagSyntaxErr struct {
	tag string
}

func (ite *InvalidTagSyntaxErr) Error() string {
	return fmt.Sprintf("Invalid syntax for tag \"%s\", allowed syntax: %s", ite.tag, tagRawRegexp)
}

type Tag string

func NewTag(s string) (Tag, error) {
	t := Tag(s)
	if err := t.Validate(); err != nil {
		return "", err
	}

	return t, nil
}

func (t Tag) Validate() error {
	s := string(t)
	if s == "" {
		return TagEmptyErr
	}

	if !tagRegex.MatchString(s) {
		return &InvalidTagSyntaxErr{s}
	}

	return nil
}