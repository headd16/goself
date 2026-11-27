package discord

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/valyala/fasthttp"
)

// FallbackBuildNumber is used when the current build number cannot be scraped
// from discord.com. Set Config.BuildNumber to override it.
const FallbackBuildNumber = 681625

var (
	// The web app embeds its build number in window.GLOBAL_ENV.
	htmlBuildRegex = regexp.MustCompile(`"BUILD_NUMBER"\s*:\s*"(\d+)"`)
	// Fallback: look into the JS bundles referenced by the page.
	scriptSrcRegex = regexp.MustCompile(`<script[^>]+src="(/assets/[^"]+\.js)"`)
	jsBuildRegexes = []*regexp.Regexp{
		regexp.MustCompile(`buildNumber["']?\s*[:=]\s*["'](\d+)["']`),
		regexp.MustCompile(`build_number["']?\s*[:=]\s*["']?(\d+)`),
		regexp.MustCompile(`Build Number: "\)\.concat\("(\d+)"`),
	}

	buildOnce   sync.Once
	cachedBuild int
)

const maxScriptsToScan = 12

// FetchBuildNumber downloads https://discord.com/app and extracts the current
// web client build number.
func FetchBuildNumber() (int, error) {
	page, err := httpGet("https://discord.com/app")
	if err != nil {
		return 0, err
	}
	if m := htmlBuildRegex.FindSubmatch(page); m != nil {
		return strconv.Atoi(string(m[1]))
	}

	scripts := scriptSrcRegex.FindAllSubmatch(page, -1)
	for i, s := range scripts {
		if i >= maxScriptsToScan {
			break
		}
		js, err := httpGet("https://discord.com" + string(s[1]))
		if err != nil {
			continue
		}
		for _, re := range jsBuildRegexes {
			if m := re.FindSubmatch(js); m != nil {
				return strconv.Atoi(string(m[1]))
			}
		}
	}
	return 0, errors.New("discord: build number not found on discord.com/app")
}

// LatestBuildNumber returns the current build number. It is fetched once per
// process and cached; FallbackBuildNumber is returned if fetching fails.
func LatestBuildNumber() int {
	buildOnce.Do(func() {
		n, err := FetchBuildNumber()
		if err != nil || n <= 0 {
			n = FallbackBuildNumber
		}
		cachedBuild = n
	})
	return cachedBuild
}

func httpGet(url string) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodGet)
	req.SetRequestURI(url)
	req.Header.SetUserAgent(DefaultUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	if err := httpClient.DoRedirects(req, resp, 5); err != nil {
		return nil, err
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("discord: GET %s returned HTTP %d", url, resp.StatusCode())
	}
	return append([]byte(nil), resp.Body()...), nil
}
