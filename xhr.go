package gosock

import (
	"encoding/json"
	"io"
	"net/http"
)

//* xhrPolling

type xhrPollingProtocol struct{}

func (p xhrPollingProtocol) contentType() string { return "application/javascript; charset=UTF-8" }

func (p xhrPollingProtocol) write(w io.Writer, m []byte) (n int, err error) {
	n, err = w.Write(append(m, '\n'))
	return
}

func (p xhrPollingProtocol) protocol() Protocol       { return ProtocolXhrPolling }
func (p xhrPollingProtocol) streaming() preludeWriter { return nil }

//* xhrStreaming

var prelude []byte = make([]byte, 2049)

func init() {
	for i := 0; i < 2048; i++ {
		prelude[i] = 'h'
	}
	prelude[2048] = '\n'
}

type xhrStreamingProtocol struct{ xhrPollingProtocol }

func (p xhrStreamingProtocol) writePrelude(w io.Writer) (err error) {
	_, err = w.Write(prelude)
	return
}

func (p xhrStreamingProtocol) protocol() Protocol       { return ProtocolXhrStreaming }
func (p xhrStreamingProtocol) streaming() preludeWriter { return p }

func xhrSendHandler(h *handler, w http.ResponseWriter, r *http.Request, sessid string) {
	var messages []string
	var decoder *json.Decoder

	header := w.Header()
	header.Add("Content-Type", "text/plain; charset=UTF-8")
	sid(h, w, r)
	xhrCors(header, r)
	noCache(header)

	s := h.pool.get(sessid)
	if s == nil {
		goto closed
	}

	decoder = json.NewDecoder(r.Body)
	if err := decoder.Decode(&messages); err != nil {
		if err == io.EOF {
			logWriteErr(h.config.Logger, w, s, "Payload expected.")
			return
		}
		logWriteErr(h.config.Logger, w, s, "Broken JSON encoding.")
		return
	}

	for _, v := range messages {
		s.rbufAppend([]byte(v))
	}

	w.WriteHeader(http.StatusNoContent)
	return

closed:
	http.NotFound(w, r)
}

func xhrOptionsHandler(h *handler, w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Add("Access-Control-Allow-Methods", "OPTIONS, POST")
	sid(h, w, r)
	xhrCors(header, r)
	enableCache(header)
	w.WriteHeader(http.StatusNoContent)
}
