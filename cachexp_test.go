package cachexp_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bukalapak/cachexp"
	"github.com/bukalapak/ottoman/cache"
	httpClone "github.com/bukalapak/ottoman/http/clone"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestExpand(t *testing.T) {
	p := NewProvider()
	h := NewRemote()
	defer h.Close()

	for _, f := range fixtureGlob("*-expandable.json") {
		t.Run(strings.TrimSuffix(f, ".json"), func(t *testing.T) {
			b := fixtureMustLoad(f)
			x := fixtureMustLoad(strings.Replace(f, "expandable", "expanded", 1))
			r := NewRequest(h.URL)

			v, err := cachexp.Expand(p, b, r)
			if err != nil {
				assert.Equal(t, x, v)
			} else {
				assert.JSONEq(t, string(x), string(v))
			}
		})
	}
}

func BenchmarkExpand(b *testing.B) {
	p := NewProvider()
	h := NewRemote()
	defer h.Close()

	for _, f := range fixtureGlob("*-expandable.json") {
		z := fixtureMustLoad(f)
		r := NewRequest(h.URL)

		b.Run(strings.TrimSuffix(f, ".json"), func(b *testing.B) {
			b.ReportAllocs()

			for n := 0; n < b.N; n++ {
				cachexp.Expand(p, z, r)
			}
		})
	}
}

func fixtureLoad(name string) ([]byte, error) {
	f, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func fixtureMustLoad(name string) []byte {
	b, err := fixtureLoad(name)
	if err != nil {
		panic(err)
	}

	return b
}

func fixtureGlob(pattern string) (gs []string) {
	if gg, err := filepath.Glob("testdata/" + pattern); err == nil {
		for i := range gg {
			gs = append(gs, filepath.Base(gg[i]))
		}
	}

	return
}

func fixtureMap(pattern string) map[string][]byte {
	m := make(map[string][]byte)

	for _, f := range fixtureGlob(pattern) {
		ss := strings.Split(strings.TrimSuffix(f, filepath.Ext(f)), "-")
		bs := []string{ss[1], ss[3], ss[2]}
		ks := strings.Join(bs, "/")

		if b, err := fixtureLoad(f); err == nil {
			m[ks] = b
		}
	}

	return m
}

type Provider struct {
	Transport http.RoundTripper
	Timeout   time.Duration
	data      map[string][]byte
}

func NewProvider() *Provider {
	p := &Provider{
		Transport: http.DefaultTransport,
		Timeout:   100 * time.Millisecond,
	}

	p.init()

	return p
}

func (p *Provider) Marshal(v interface{}) ([]byte, error)   { return jsoniter.Marshal(v) }
func (p *Provider) Unmarshal(b []byte, v interface{}) error { return jsoniter.Unmarshal(b, v) }

func (p *Provider) init() {
	p.data = make(map[string][]byte)

	for k, v := range fixtureMap("cache-v?-??-*.json") {
		p.data[p.Normalize(k)] = v
	}
}

func (p *Provider) read(key string) ([]byte, error) {
	if b, ok := p.data[p.Normalize(key)]; ok {
		return b, nil
	}

	return nil, errors.New("cache does not exist")
}

func (p *Provider) readMulti(keys []string) (map[string][]byte, error) {
	mx := make(map[string][]byte, len(keys))

	for _, s := range keys {
		if b, err := p.read(s); err == nil {
			mx[s] = b
		}
	}

	return mx, nil
}

func (p *Provider) httpClient() *http.Client {
	return &http.Client{
		Transport: p.Transport,
		Timeout:   p.Timeout,
	}
}

func (p *Provider) IsExcluded(key string) bool {
	return strings.HasPrefix(key, "__")
}

func (p *Provider) Normalize(key string) string {
	return cache.Normalize(key, "prefix")
}

func (p *Provider) Fetch(key string, r *http.Request) ([]byte, error) {
	if b, err := p.read(key); err == nil {
		return b, nil
	}

	req, err := p.Resolve(key, r)
	if err != nil {
		return nil, err
	}

	c := p.httpClient()

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("invalid http status:" + res.Status)
	}

	return ioutil.ReadAll(res.Body)
}

func (p *Provider) Resolve(key string, r *http.Request) (*http.Request, error) {
	req := httpClone.Request(r)
	req.URL.Path = "/" + key

	return req, nil
}

func NewRemote() *httptest.Server {
	rm := fixtureMap("remote-v?-??-*.json")
	fn := func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")

		if b, ok := rm[key]; ok {
			w.Write(b)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}

	return httptest.NewServer(http.HandlerFunc(fn))
}

func NewRequest(s string) *http.Request {
	r, _ := http.NewRequest("GET", s, nil)
	return r
}
