package types

import (
	"strconv"
	"strings"
)

// FlexString decodes a JSON string, number or null into a string. Discord
// sends some fields (such as message nonces) as either strings or integers.
type FlexString string

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "null" || s == "":
		*f = ""
	case s[0] == '"':
		v, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		*f = FlexString(v)
	default:
		*f = FlexString(s)
	}
	return nil
}

// String returns the underlying string.
func (f FlexString) String() string { return string(f) }

// FlexInt decodes a JSON number, numeric string or null into an int64.
type FlexInt int64

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" || s == `""` {
		*f = 0
		return nil
	}
	if s[0] == '"' {
		unq, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		s = unq
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		*f = FlexInt(v)
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = FlexInt(v)
	return nil
}
