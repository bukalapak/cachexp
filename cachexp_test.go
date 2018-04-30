package cachexp_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bukalapak/cachexp"
	"github.com/bukalapak/ottoman/cache"
	"github.com/bukalapak/ottoman/encoding/json"
	httpclone "github.com/bukalapak/ottoman/http/clone"
	"github.com/stretchr/testify/assert"
)

func TestExpand(t *testing.T) {
	p := NewProvider()
	h := NewRemote()
	defer h.Close()

	for _, f := range fixtureGlob("*-expandable.json") {
		t.Run(strings.TrimSuffix(f, ".json"), func(t *testing.T) {
			z := fixtureMustLoad(f)
			x := fixtureMustLoad(strings.Replace(f, "expandable", "expanded", 1))
			r, _ := http.NewRequest("GET", h.URL, nil)

			v, err := cachexp.Expand(p, z, r)
			if err != nil {
				assert.Equal(t, x, v)
			} else {
				JSONEq(t, x, v)
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
		r, _ := http.NewRequest("GET", h.URL, nil)

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

func JSONEq(t *testing.T, a, b []byte) {
	var x interface{}
	var y interface{}

	if err := json.Unmarshal(a, &x); err != nil {
		assert.Nil(t, err)
	}

	if err := json.Unmarshal(b, &y); err != nil {
		assert.Nil(t, err)
	}

	compare(t, x, y)
}

func compare(t *testing.T, x, y interface{}) {
	switch z := x.(type) {
	case map[string]interface{}:
		for k, v := range z {
			compare(t, v, y.(map[string]interface{})[k])
		}
	case []interface{}:
		assert.ElementsMatch(t, z, y)
	default:
		assert.Equal(t, x, y)
	}
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

type Provider struct {
	N *cache.Engine
	t cachexp.Tuner
}

func NewProvider() cachexp.Provider {
	g := NewStorage("prefix")
	p := cache.NewProvider(g).(*cache.Engine)
	p.Prefix = g.prefix
	p.Resolver = NewResolver()

	return &Provider{N: p, t: NewTuner()}
}

func (p *Provider) Marshal(v interface{}) ([]byte, error)   { return json.Marshal(v) }
func (p *Provider) Unmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }
func (p *Provider) Tuner() cachexp.Tuner                    { return p.t }

func (p *Provider) ReadFetch(key string, r *http.Request) ([]byte, error) {
	return p.N.ReadFetch(p.N.Normalize(key), r)
}

func (p *Provider) ReadFetchMulti(keys []string, r *http.Request) (map[string][]byte, error) {
	return p.N.ReadFetchMulti(p.N.NormalizeMulti(keys), r)
}

type Storage struct {
	data   map[string][]byte
	prefix string
}

func NewStorage(prefix string) *Storage {
	data := make(map[string][]byte)

	for k, v := range fixtureMap("cache-v?-??-*.json") {
		data[cache.Normalize(k, prefix)] = v
	}

	return &Storage{data: data, prefix: prefix}
}

func (g *Storage) Name() string {
	return "cachexp-storage"
}

func (g *Storage) Read(key string) ([]byte, error) {
	if b, ok := g.data[key]; ok {
		return b, nil
	}

	return nil, errors.New("cache does not exist")
}

func (g *Storage) ReadMulti(keys []string) (map[string][]byte, error) {
	mx := make(map[string][]byte, len(keys))

	for _, s := range keys {
		if b, err := g.Read(s); err == nil {
			mx[s] = b
		}
	}

	return mx, nil
}

type Resolver struct{}

func NewResolver() cache.Resolver { return &Resolver{} }

func (m *Resolver) Resolve(key string, r *http.Request) (*http.Request, error) {
	req := httpclone.Request(r)
	req.URL.Path = "/" + cache.Normalize(key, "")

	return req, nil
}

func (m *Resolver) ResolveRequest(r *http.Request) (*http.Request, error) {
	return httpclone.Request(r), nil
}

type Tuner struct{}

func NewTuner() *Tuner { return &Tuner{} }

func (n *Tuner) ExpandKey() string          { return "__cache_keys" }
func (n *Tuner) PlaceholderKey() string     { return "__" }
func (n *Tuner) ExpandDepth() int           { return 4 }
func (n *Tuner) IsExcluded(key string) bool { return strings.HasPrefix(key, "__") }
