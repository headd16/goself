package discord

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jeandeaual/go-locale"
)

var (
	localeOnce   sync.Once
	systemLocale string
)

// SystemLocale returns the operating system locale as a BCP 47 tag such as
// "en-US", or "en-US" if it cannot be detected.
func SystemLocale() string {
	localeOnce.Do(func() {
		l, err := locale.GetLocale()
		systemLocale = normalizeLocale(l)
		if err != nil || systemLocale == "" {
			systemLocale = "en-US"
		}
	})
	return systemLocale
}

func normalizeLocale(l string) string {
	l = strings.TrimSpace(l)
	if i := strings.IndexAny(l, ".@"); i >= 0 { // en_US.UTF-8, de_DE@euro
		l = l[:i]
	}
	l = strings.ReplaceAll(l, "_", "-")
	if l == "C" || l == "POSIX" {
		return ""
	}
	return l
}

// SystemTimezone returns the IANA time zone of the machine (from $TZ or the
// local zone name), or "UTC" if it cannot be determined.
func SystemTimezone() string {
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); tz != "" && strings.Contains(tz, "/") {
		return tz
	}
	if name := time.Local.String(); strings.Contains(name, "/") {
		return name
	}
	return "UTC"
}
