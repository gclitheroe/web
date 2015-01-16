// Provides http handlers for writing responses to clients.  Metrics and logging about requests and reponses.
//
// The following metrics are exposed via expvar
//  * requests.rate - requests per s.  Updated every 20s.
//  * requests.time - average time (s) to handle request.  Updated every 20s.
//  * requests.timeDB - average time (s) to handle DB request.  Updated every 20s.
//  * responses.2xx - 2xx responses per s.  Updated every 20s.
//  * responses.4xx - 4xx responses per s.  Updated every 20s.
//  * responses.5xx - 5xx responses per s.  Updated every 20s.
//
// The requests.timeDB metric must be updated by your application.  If your app
// doesn't use a DB then you can safely ignore this (the counter will stay at 0).
// Update the metric via the exposed DBTime e.g.,
//
//    start := time.Now()
//    ... do database access stuff...
//    web.DBTime.Inc(start)
//
package web

import (
	"bytes"
	"expvar"
	"github.com/GeoNet/app/metrics"
	"log"
	"net/http"
	"time"
)

// For setting Cache-Control and Surrogate-Control headers.
const (
	MaxAge10    = "max-age=10"
	MaxAge300   = "max-age=300"
	MaxAge86400 = "max-age=86400"
)

// These constants represent part of a public API and can't be changed.
const (
	V1GeoJSON = "application/vnd.geo+json;version=1"
	V1JSON    = "application/json;version=1"
	V1CSV     = "text/csv;version=1"
)

// These constants are for error and other pages.  They can be changed.
const (
	ErrContent  = "text/plain; charset=utf-8"
	HtmlContent = "text/html; charset=utf-8"
)

// metrics
var (
	req     = expvar.NewMap("requests")
	res     = expvar.NewMap("responses")
	r2xx    metrics.Rate
	r4xx    metrics.Rate
	r5xx    metrics.Rate
	reqRate metrics.Rate
	reqTime metrics.Timer
	DBTime  metrics.Timer
)

type Header struct {
	Cache, Surrogate string // Set as the default in the response header - can override in handler funcs.
	Vary             string // This is added to the response header (which may already Vary on gzip).
}

func init() {
	req.Init()
	res.Init()
	r2xx.Init(time.Duration(1)*time.Second, time.Duration(20)*time.Second)
	r4xx.Init(time.Duration(1)*time.Second, time.Duration(20)*time.Second)
	r5xx.Init(time.Duration(1)*time.Second, time.Duration(20)*time.Second)
	reqRate.Init(time.Duration(1)*time.Second, time.Duration(20)*time.Second)
	reqTime.Init(time.Duration(20) * time.Second)
	DBTime.Init(time.Duration(20) * time.Second)

	req.Set("rate", &reqRate)
	req.Set("time", &reqTime)
	req.Set("timeDB", &DBTime)

	res.Set("2xx", &r2xx)
	res.Set("4xx", &r4xx)
	res.Set("5xx", &r5xx)
}

// OkBuf (200) - writes the content in the bytes.Buffer pointed to by b to w.
// Using a Buffer is useful for avoiding writing partial content to the client
// if an error could occur when generating the content.
func OkBuf(w http.ResponseWriter, r *http.Request, b *bytes.Buffer) {
	// Haven't bothered logging 200s.
	r2xx.Inc()
	b.WriteTo(w)
}

// Ok (200) - writes the content in the []byte pointed by b to w.
func Ok(w http.ResponseWriter, r *http.Request, b *[]byte) {
	// Haven't bothered logging 200s.
	r2xx.Inc()
	w.Write(*b)
}

// OkTrack (200) - increments the response 2xx counter and nothing
// else.
func OkTrack(w http.ResponseWriter, r *http.Request) {
	// Haven't bothered logging 200s.
	r2xx.Inc()
}

// NotFound (404) - whatever the client was looking for we haven't got it.  The message should try
// to explain why we couldn't find that thing that they was looking for.
// Use for things that might become available.
func NotFound(w http.ResponseWriter, r *http.Request, message string) {
	log.Println(r.RequestURI + " 404")
	r4xx.Inc()
	w.Header().Set("Cache-Control", MaxAge10)
	w.Header().Set("Surrogate-Control", MaxAge10)
	http.Error(w, message, http.StatusNotFound)
}

// NotFoundPage (404) - returns a 404 html error page.
// Whatever the client was looking for we haven't got it.
func NotFoundPage(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RequestURI + " 404")
	r4xx.Inc()
	w.Header().Set("Cache-Control", MaxAge10)
	w.Header().Set("Surrogate-Control", MaxAge10)
	w.WriteHeader(http.StatusNotFound)
	w.Write(error404)
}

// NotAcceptable (406) - the client requested content we don't know how to
// generate. The message should suggest content types that can be created.
func NotAcceptable(w http.ResponseWriter, r *http.Request, message string) {
	log.Println(r.RequestURI + " 406")
	r4xx.Inc()
	w.Header().Set("Cache-Control", MaxAge10)
	w.Header().Set("Surrogate-Control", MaxAge86400)
	http.Error(w, message, http.StatusNotAcceptable)
}

// MethodNotAllowed - the client used a method we don't allow.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RequestURI + " 405")
	r4xx.Inc()
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// BadRequest (400) the client made a badRequest request that should not be repeated without correcting it.
// Message should explain what is bad about the request.
// Use for things that will never become available.
func BadRequest(w http.ResponseWriter, r *http.Request, message string) {
	log.Println(r.RequestURI + " 400")
	r4xx.Inc()
	w.Header().Set("Cache-Control", MaxAge10)
	w.Header().Set("Surrogate-Control", MaxAge86400)
	http.Error(w, message, http.StatusBadRequest)
}

// ServiceUnavailable (503) - some sort of internal server error.
func ServiceUnavailable(w http.ResponseWriter, r *http.Request, err error) {
	log.Println(r.RequestURI + " 503")
	log.Printf("ERROR %s", err)
	r5xx.Inc()
	http.Error(w, "Sad trombone.  Something went wrong and for that we are very sorry.  Please try again in a few minutes.", http.StatusServiceUnavailable)
}

// ServiceUnavailablePage (500) - returns a 500 error page.
func ServiceUnavailablePage(w http.ResponseWriter, r *http.Request, err error) {
	log.Println(r.RequestURI + " 503")
	log.Printf("ERROR %s", err)
	r5xx.Inc()
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write(error503)
}

// GetAPI creates an http handler that only responds to http GET requests.  All other methods are an error.
// Sets default Cache-Control and Surrogate-Control headers.
// Sets the Vary header to Accept for use with REST APIs and upstream caching.
// Increments the request counter.
// Tracks response times.
func (hdr *Header) Get(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqRate.Inc()
		if r.Method == "GET" {
			defer reqTime.Inc(time.Now())
			log.Printf("GET %s", r.URL)
			w.Header().Set("Cache-Control", hdr.Cache)
			w.Header().Set("Surrogate-Control", hdr.Surrogate)
			w.Header().Add("Vary", hdr.Vary)
			h.ServeHTTP(w, r)
			return
		}
		MethodNotAllowed(w, r)
	})
}

func (hdr *Header) GetGzip(m *http.ServeMux) http.Handler {
	return hdr.Get(GzipHandler(m))
}
