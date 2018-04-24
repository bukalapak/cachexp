package cachexp_test

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bukalapak/cachexp"
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

	for _, f := range fixtureGlob("*-expandable.json") {
		z := fixtureMustLoad(f)

		b.ReportAllocs()
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

type provider struct{}

func (p *provider) Marshal(v interface{}) ([]byte, error)   { return json.Marshal(v) }
func (p *provider) Unmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }

func (p *provider) Read(key string) ([]byte, error) {
	ms := map[string]string{}

	for _, f := range fixtureGlob("*-v?-??-*.json") {
		ss := strings.Split(strings.TrimSuffix(f, filepath.Ext(f)), "-")
		bs := []string{ss[1], ss[3], ss[2]}

		ms[strings.Join(bs, "/")] = f
	}

	return fixtureLoad(ms[key])
}

func (p *provider) ReadMulti(keys []string) (map[string][]byte, error) {
	mx := make(map[string][]byte, len(keys))

	for _, s := range keys {
		b, err := p.Read(s)
		if err != nil {
			return nil, err
		}

		mx[s] = b
	}

	return mx, nil
}
