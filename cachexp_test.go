package cachexp_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bukalapak/cachexp"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestExpand(t *testing.T) {
	p := &provider{}

	for _, f := range fixtureGlob("*-expandable.json") {
		t.Run(strings.TrimSuffix(f, ".json"), func(t *testing.T) {
			b := fixtureMustLoad(f)
			x := fixtureMustLoad(strings.Replace(f, "expandable", "expanded", 1))

			v, err := cachexp.Expand(p, b)
			assert.Nil(t, err)
			assert.JSONEq(t, string(x), string(v))
		})
	}
}

func BenchmarkExpand(b *testing.B) {
	p := &provider{}
	b.ReportAllocs()

	for _, f := range fixtureGlob("*-expandable.json") {
		z := fixtureMustLoad(f)

		b.Run(strings.TrimSuffix(f, ".json"), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				cachexp.Expand(p, z)
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

type provider struct {
	once sync.Once
	data map[string][]byte
}

func (p *provider) Marshal(v interface{}) ([]byte, error)   { return jsoniter.Marshal(v) }
func (p *provider) Unmarshal(b []byte, v interface{}) error { return jsoniter.Unmarshal(b, v) }

func (p *provider) init() {
	p.data = make(map[string][]byte)

	for _, f := range fixtureGlob("*-v?-??-*.json") {
		ss := strings.Split(strings.TrimSuffix(f, filepath.Ext(f)), "-")
		bs := []string{ss[1], ss[3], ss[2]}
		ks := strings.Join(bs, "/")

		if b, err := fixtureLoad(f); err == nil {
			p.data[ks] = b
		}
	}
}

func (p *provider) Read(key string) ([]byte, error) {
	p.once.Do(p.init)
	return p.data[key], nil
}

func (p *provider) ReadMulti(keys []string) (map[string][]byte, error) {
	mx := make(map[string][]byte, len(keys))

	for _, s := range keys {
		if b, err := p.Read(s); err == nil {
			mx[s] = b
		}
	}

	return mx, nil
}
