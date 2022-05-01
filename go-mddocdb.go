package mddocdb

import (
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/hashicorp/go-memdb"
)

var (
	extensions = parser.CommonExtensions | parser.AutoHeadingIDs | parser.NonBlockingSpace
	titleregex = regexp.MustCompile(`<h1 id=".*?">(.*?)</h1>`)
)

type Options struct {
	CacheDuration time.Duration
	Template      *template.Template
	Logger        *log.Logger
}

type FS interface {
	OpenRead(key string) (io.Reader, error)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func respond(w http.ResponseWriter, statusCode int) {
	w.WriteHeader(statusCode)
	w.Write([]byte(http.StatusText(statusCode)))
}

type docTxn struct {
	*memdb.Txn
}

type CachedDocument struct {
	Key       string
	Document  []byte
	CreatedAt time.Time
}

func (d docTxn) get(key string) (*CachedDocument, error) {
	doc, err := d.First("docs", "id", key)
	if err != nil && err != memdb.ErrNotFound {
		return nil, err
	} else if doc == nil {
		return nil, nil
	} else {
		return doc.(*CachedDocument), nil
	}
}

func (d docTxn) replace(before *CachedDocument, after *CachedDocument) error {
	if before != nil {
		err := d.Delete("docs", before)
		if err != nil && err != memdb.ErrNotFound {
			return err
		}
	}
	return d.Insert("docs", after)
}

func GetHandler(fs FS, basepath string, opts *Options) http.Handler {
	db, err := memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"docs": &memdb.TableSchema{
				Name: "docs",
				Indexes: map[string]*memdb.IndexSchema{
					"id": &memdb.IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Key"},
					},
				},
			},
		},
	})
	must(err)
	l := opts.Logger
	if l == nil {
		l = log.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if key[:len(basepath)] == basepath {
			key = key[len(basepath):]
		}
		key = key + ".md"
		if key[0] == '/' {
			key = key[1:]
		}
		tx := docTxn{db.Txn(true)}
		defer tx.Commit()
		doc, err := tx.get(key)
		if err != nil {
			l.Printf("error fetching document from cache: %s", err.Error())
			respond(w, http.StatusInternalServerError)
			return
		} else if doc != nil && time.Now().Sub(doc.CreatedAt) < opts.CacheDuration {
			// already in cache and usable
			w.Header().Set("Server-Timing", "cachehit")
			w.WriteHeader(http.StatusOK)
			w.Write(doc.Document)
			return
		}

		// not in cache; fetch from FS, replace
		reader, err := fs.OpenRead(key)
		if err != nil {
			l.Printf("error fetching file from storage: %s", err.Error())
			respond(w, http.StatusInternalServerError)
			return
		} else if reader == nil {
			l.Printf("file not found in storage: %s", key)
			respond(w, http.StatusNotFound)
			return
		}
		bytes, err := io.ReadAll(reader)
		if err != nil {
			l.Printf("error reading data from file: %s", err.Error())
			respond(w, http.StatusInternalServerError)
			return
		}
		html := markdown.ToHTML(bytes, parser.NewWithExtensions(extensions), nil)

		match := titleregex.FindSubmatch(html)
		title := ""
		if match != nil && len(match) > 1 {
			title = string(match[1])
		}

		sb := &strings.Builder{}
		err = opts.Template.Execute(sb, struct {
			Document string
			Title    string
		}{
			Document: string(html),
			Title:    title,
		})
		if err != nil {
			l.Printf("error rendering template: %s", err.Error())
			respond(w, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Server-Timing", "cachemiss")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sb.String()))
		doc2 := &CachedDocument{
			Key:       key,
			Document:  []byte(sb.String()),
			CreatedAt: time.Now(),
		}
		err = tx.replace(doc, doc2)
		if err != nil {
			l.Printf("error rendering updating cache: %s", err.Error())
			respond(w, http.StatusInternalServerError)
			return
		}
	})
}
