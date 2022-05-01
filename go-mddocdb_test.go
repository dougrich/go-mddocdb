package mddocdb_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/dougrich/go-mddocdb"
	"github.com/stretchr/testify/assert"
)

var (
	bodyTemplate = template.Must(template.New("body").Parse("<head>{{if .Title}}<title>{{.Title}}</title>{{end}}</head><body>{{.Document}}</body>"))
	testFS       = &TestFS{}
)

func TestHandler(t *testing.T) {
	assert := assert.New(t)
	handler := mddocdb.GetHandler(testFS, "/docs", &mddocdb.Options{
		CacheDuration: 100 * time.Millisecond, // how long to keep the cache around for
		Template:      bodyTemplate,           // the template to use for the html page
	})
	testFS.set("example.md", "# Hello World\nFancy")

	// check OK status
	{
		req, err := http.NewRequest("GET", "/docs/example", nil)
		assert.NoError(err)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		result := rr.Result()
		body := rr.Body.String()

		assert.Equal(http.StatusOK, rr.Code)
		assert.Equal("cachemiss", result.Header.Get("Server-Timing"))
		assert.Equal("<head><title>Hello World</title></head><body><h1 id=\"hello-world\">Hello World</h1>\n\n<p>Fancy</p>\n</body>", body)
	}

	// check cache status
	{
		req, err := http.NewRequest("GET", "/docs/example", nil)
		assert.NoError(err)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		result := rr.Result()
		body := rr.Body.String()

		assert.Equal(http.StatusOK, rr.Code)
		assert.Equal("cachehit", result.Header.Get("Server-Timing"))
		assert.Equal("<head><title>Hello World</title></head><body><h1 id=\"hello-world\">Hello World</h1>\n\n<p>Fancy</p>\n</body>", body)
	}

	// check not found
	{
		req, err := http.NewRequest("GET", "/docs/bad", nil)
		assert.NoError(err)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(http.StatusNotFound, rr.Code)
	}

	// check cache expiry
	{
		time.Sleep(100 * time.Millisecond)
		req, err := http.NewRequest("GET", "/docs/example", nil)
		assert.NoError(err)
		rr := httptest.NewRecorder()
		testFS.body = "# Changed\nWhat a world"

		handler.ServeHTTP(rr, req)
		result := rr.Result()
		body := rr.Body.String()

		assert.Equal(http.StatusOK, rr.Code)
		assert.Equal("cachemiss", result.Header.Get("Server-Timing"))
		assert.Equal("<head><title>Changed</title></head><body><h1 id=\"changed\">Changed</h1>\n\n<p>What a world</p>\n</body>", body)
	}
}

type TestFS struct {
	key   string
	body  string
	count int
}

func (t *TestFS) set(key string, body string) {
	t.key = key
	t.body = body
}

func (t *TestFS) OpenRead(name string) (io.Reader, error) {
	t.count = t.count + 1
	if name == t.key {
		return strings.NewReader(t.body), nil
	} else {
		return nil, nil
	}
}
