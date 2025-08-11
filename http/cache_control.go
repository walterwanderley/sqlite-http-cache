package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// directives
const (
	directiveMaxAge          = "max-age"
	directiveMaxStale        = "max-stale"
	directiveMinFresh        = "min-fresh"
	directiveNoCache         = "no-cache"
	directiveNoStore         = "no-store"
	directiveNoTransform     = "no-transform"
	directiveOnlyIfCached    = "only-if-cached"
	directiveMustRevalidate  = "must-revalidate"
	directiveMustUnderstand  = "must-understand"
	directivePrivate         = "private"
	directiveProxyRevalidate = "proxy-revalidate"
	directivePublic          = "public"
	directiveSMaxAge         = "s-maxage"
)

type CacheControl struct {
	maxAge          *int
	maxStale        *int
	minFresh        *int
	noCache         bool
	noStore         bool
	noTransform     bool
	onlyIfCached    bool
	mustRevalidate  bool
	mustUnderstand  bool
	private         bool
	proxyRevalidate bool
	public          bool
	sMaxAge         *int

	sharedCache bool

	date    *time.Time
	expires *time.Time

	requestTime *time.Time
}

func ParseCacheControl(header http.Header, requestTime *time.Time, sharedCache bool) (cc CacheControl) {
	cc.requestTime = requestTime
	cc.sharedCache = sharedCache
	cc.requestTime = requestTime

	cacheControlHeader := header.Get("Cache-Control")

	cacheControlHeader = strings.ToLower(strings.ReplaceAll(cacheControlHeader, " ", ""))
	if cacheControlHeader == "" {
		return
	}

	directives := strings.Split(cacheControlHeader, ",")
	for _, d := range directives {
		splited := strings.SplitN(d, "=", 2)
		switch len(splited) {
		case 1:
			switch splited[0] {
			case directiveNoCache:
				cc.noCache = true
			case directiveNoStore:
				cc.noStore = true
			case directiveOnlyIfCached:
				cc.onlyIfCached = true
			case directiveMustRevalidate:
				cc.mustRevalidate = true
			case directiveMustUnderstand:
				cc.mustUnderstand = true
			case directiveNoTransform:
				cc.noTransform = true
			case directivePrivate:
				cc.private = true
			case directiveProxyRevalidate:
				cc.proxyRevalidate = true
			case directivePublic:
				cc.public = true
			}
		case 2:
			k := splited[0]
			v, _ := strconv.Atoi(strings.TrimSpace(splited[1]))
			switch k {
			case directiveMaxAge:
				cc.maxAge = &v
			case directiveMaxStale:
				cc.maxStale = &v
			case directiveMinFresh:
				cc.minFresh = &v
			case directiveSMaxAge:
				cc.sMaxAge = &v
			}
		}
	}

	date := header.Get("Date")
	if date != "" {
		t, err := time.Parse(time.RFC1123, date)
		if err == nil {
			cc.date = &t
		}
	}

	expires := header.Get("Expires")
	if expires != "" {
		t, err := time.Parse(time.RFC1123, expires)
		if err == nil {
			cc.expires = &t
		}
	}

	return
}

func (c CacheControl) Cacheable() bool {
	switch {
	case c.noCache:
		return false
	case c.noStore:
		return false
	case c.private:
		return !c.sharedCache
	default:
		return true
	}
}

func (c CacheControl) Expired() bool {
	if !c.Cacheable() {
		return true
	}

	var refDate *time.Time
	switch {
	case c.date != nil:
		refDate = c.date
	case c.requestTime != nil:
		refDate = c.requestTime
	}
	if refDate == nil {
		return true
	}

	age := int(time.Since(*refDate).Seconds())

	return c.isStale(age)
}

func (c CacheControl) isStale(age int) bool {
	freshnessLifetime := c.FreshnessLifetime()
	if freshnessLifetime == nil {
		return true
	}
	return *freshnessLifetime < age
}

func (c CacheControl) MaxAge() *int {
	return c.maxAge
}

func (c CacheControl) MaxStale() *int {
	return c.maxStale
}

func (c CacheControl) MinFresh() *int {
	return c.minFresh
}

func (c CacheControl) NoCache() bool {
	return c.noCache
}

func (c CacheControl) NoStore() bool {
	return c.noStore
}

func (c CacheControl) NoTransform() bool {
	return c.noTransform
}

func (c CacheControl) OnlyIfCached() bool {
	return c.onlyIfCached
}

func (c CacheControl) MustRevalidate() bool {
	return c.mustRevalidate
}

func (c CacheControl) MustUnderstand() bool {
	return c.mustUnderstand
}

func (c CacheControl) Private() bool {
	return c.private
}

func (c CacheControl) ProxyRevalidate() bool {
	return c.proxyRevalidate
}

func (c CacheControl) Public() bool {
	return c.public
}

func (c CacheControl) SMaxAge() *int {
	return c.sMaxAge
}

func (c CacheControl) FreshnessLifetime() *int {
	switch {
	case c.sharedCache && c.sMaxAge != nil:
		return c.sMaxAge
	case c.maxAge != nil:
		return c.maxAge
	case c.expires != nil:
		if c.date != nil {
			t := int(c.expires.Sub(*c.date).Seconds())
			if t >= 0 {
				return &t
			}
		} else if c.requestTime != nil {
			t := int(c.expires.Sub(*c.requestTime).Seconds())
			if t >= 0 {
				return &t
			}
		}
	}

	return nil
}
