package gosock

import (
	"net/http"
	"regexp"
)

var reGreeting = regexp.MustCompile(`^/?$`)
var reInfo = regexp.MustCompile(`^/info$`)
var reIframe = regexp.MustCompile(`^/iframe[\w\d-\. ]*\.html$`)
var reSessionUrl = regexp.MustCompile(
	`^/(?:[\w- ]+)/([\w- ]+)/(xhr|xhr_send|xhr_streaming|eventsource|htmlfile|websocket|jsonp|jsonp_send)$`)
var reRawWebsocket = regexp.MustCompile(`^/websocket$`)

type handler struct {
	prefix string
	hfunc  func(Session)
	config *Config
	pool   *legacyPool
}

func newHandler(prefix string, hfunc func(Session), c *Config) http.Handler {
	h := new(handler)
	h.prefix = prefix
	h.hfunc = hfuncCloseWrapper(hfunc)
	h.config = c
	h.pool = newLegacyPool()
	return h
}

// NewHandler creates a new SockJS handler with the given
// prefix, handler function and configuration.
func NewHandler(prefix string, hfunc func(Session), c Config) http.Handler {
	if len(prefix) > 0 && prefix[len(prefix)-1] == '/' {
		panic("prefix must not end with a slash")
	}

	h := newHandler(prefix, hfunc, &c)
	f := func(w http.ResponseWriter, r *http.Request) {
		if pathMatch(prefix, r.URL.Path) {
			h.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	}
	return http.HandlerFunc(f)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len(h.prefix):]
	method := r.Method
	header := w.Header()

	switch {
	case method == "GET" && reGreeting.MatchString(path):
		greetingHandler(w)
	case method == "GET" && reIframe.MatchString(path):
		iframeHandler(h, w, r)
	case method == "OPTIONS" && reInfo.MatchString(path):
		infoOptionsHandler(h, w, r)
	case method == "GET" && reInfo.MatchString(path):
		infoHandler(h, w, r)
	case method == "GET" && reRawWebsocket.MatchString(path):
		rawWebsocketHandler(h, w, r)
	case method == "GET" && reSessionUrl.MatchString(path):
		matches := reSessionUrl.FindStringSubmatch(path)
		sessid := matches[1]
		protocol := matches[2]
		switch protocol {
		case "websocket":
			websocketHandler(h, w, r, sessid)
		case "eventsource":
			legacyHandler(h, w, r, sessid, eventSourceProtocol{})
		case "htmlfile":
			htmlfileHandler(h, w, r, sessid)
		case "jsonp":
			jsonpHandler(h, w, r, sessid)
		}
	case method == "POST" && reSessionUrl.MatchString(path):
		matches := reSessionUrl.FindStringSubmatch(path)
		sessid := matches[1]
		protocol := matches[2]
		switch protocol {
		case "websocket":
			websocketPostHandler(w, r)
		case "xhr":
			xhrCors(header, r)
			legacyHandler(h, w, r, sessid, xhrPollingProtocol{})
		case "xhr_streaming":
			xhrCors(header, r)
			legacyHandler(h, w, r, sessid, xhrStreamingProtocol{})
		case "xhr_send":
			xhrCors(header, r)
			xhrSendHandler(h, w, r, sessid)
		case "jsonp_send":
			jsonpSendHandler(h, w, r, sessid)
		}
	case method == "OPTIONS" && reSessionUrl.MatchString(path):
		xhrOptionsHandler(h, w, r)
	default:
		http.NotFound(w, r)
	}
}
