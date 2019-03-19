package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"regexp"
	"time"

	"github.com/JoveYu/zgo/log"
)

const (
	PoweredBy string = "zgo/0.0.1"
)

type Server struct {
	Addr    string
	Routers []Router
	Logger  *log.LevelLogger
	Charset string
	Debug   bool
}

func NewServer() *Server {
	server := Server{
		Logger:  log.DefaultLog,
		Charset: "utf-8",
		Debug:   false,
	}
	return &server
}

func (s *Server) Router(method string, path string, handlers ...ContextHandlerFunc) {
	cr, err := regexp.Compile(path)
	if err != nil {
		s.Logger.Warn("can not add route [%s] %s", path, err)
		return
	}

	s.Routers = append(s.Routers, Router{
		r:        path,
		cr:       cr,
		method:   method,
		handlers: handlers,
	})

}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tstart := time.Now()

	ctx := NewContext(w, r)
	defer s.LogRequest(tstart, &ctx)

	// debug
	if s.Debug {
		// debug req
		data, err := httputil.DumpRequest(r, true)
		if err != nil {
			s.Logger.Error("can not dump req: %s", err)
		}
		for _, b := range bytes.Split(data, []byte("\n")) {
			s.Logger.Debug("> %s", b)
		}

		// debug resp
		rec := httptest.NewRecorder()
		ctx = NewContext(rec, r)

		defer func() {
			s.Logger.Debug("< %s %d %s", r.Proto, rec.Code, http.StatusText(rec.Code))
			for k, v := range rec.HeaderMap {
				w.Header()[k] = v
				for _, vv := range v {
					log.Debug("< %s: %s", k, vv)
				}
			}

			s.Logger.Debug("<")
			s.Logger.Debug("< %s", rec.Body)
			w.WriteHeader(rec.Code)
			rec.Body.WriteTo(w)

		}()
	}

	path := ctx.URL().Path

	// default header
	ctx.SetHeader("X-Powered-By", PoweredBy)
	ctx.SetContentType("text/plain")

	for _, router := range s.Routers {
		if ctx.Method() != router.method {
			continue
		}

		if !router.cr.MatchString(path) {
			continue
		}

		match := router.cr.FindStringSubmatch(path)
		if len(match[0]) != len(path) {
			continue
		}

		if len(match) > 1 {
			for idx, name := range router.cr.SubexpNames()[1:] {
				ctx.Params[name] = match[idx+1]
			}
			log.Debug(ctx.Params)
		}

		for _, h := range router.handlers {
			h(ctx)
			// if WriteHeader then break next
			if ctx.ResponseWriter.IsWrited() {
				break
			}
		}
		return
	}

	ctx.Abort(http.StatusNotFound, http.StatusText(http.StatusNotFound))
}

func (s *Server) Run(addr string) error {
	if s.Logger == nil {
		s.Logger = log.DefaultLog
	}

	return http.ListenAndServe(addr, s)
}

func (s *Server) LogRequest(tstart time.Time, ctx *Context) {

	s.Logger.Info("%d|%s|%s|%s|%s|%d",
		ctx.ResponseWriter.status, ctx.Method(), ctx.URL().Path,
		ctx.Query().Encode(), ctx.ClientIP(),
		time.Since(tstart)/time.Microsecond,
	)
}
