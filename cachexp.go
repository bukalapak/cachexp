// Package cachexp provides cache expansion mechanism using selected Provider.
package cachexp

import (
	"net/http"
)

// Tuner allows cutomization during the expansion.
type Tuner interface {
	ExpandKey() string
	PlaceholderKey() string
	ExpandDepth() int
	IsExcluded(key string) bool
}

// A Provider provides data and transformation for the Expand function.
// It's also provides Tuner to customize behaviour of the expansion.
type Provider interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(b []byte, v interface{}) error

	ReadFetch(key string, r *http.Request) ([]byte, error)
	ReadFetchMulti(keys []string, r *http.Request) (map[string][]byte, error)

	Normalize(key string) string
	Tuner() Tuner
}

// Expand process expansion of the b using selected Provider.
// When r provided, it can be used by Provider to fetch data
// from remote backend when no local data is not exist during expansion.
func Expand(c Provider, b []byte, r *http.Request) ([]byte, error) {
	var v interface{}

	if err := c.Unmarshal(b, &v); err != nil {
		return b, err
	}

	switch m := v.(type) {
	case map[string]interface{}:
		return c.Marshal(expand(c, m, c.Tuner().ExpandDepth(), r))
	}

	return b, nil
}

func expand(c Provider, m map[string]interface{}, n int, r *http.Request) interface{} {
	d := n
	y := []map[string]interface{}{}
	x := make(map[string]interface{})

	for k, v := range m {
		if k == c.Tuner().ExpandKey() {
			d--

			if d >= 0 {
				switch t := v.(type) {
				case map[string]interface{}:
					for kk, vv := range childMap(c, t, d, r) {
						x[kk] = vv
					}
				case []interface{}:
					y = append(y, childSlice(c, t, d, r)...)
				}
			}

			continue
		}

		switch t := v.(type) {
		case map[string]interface{}:
			x[k] = expand(c, t, d, r)
		default:
			x[k] = v
		}
	}

	if len(y) != 0 && len(x) != 0 {
		x[c.Tuner().PlaceholderKey()] = y
		return x
	}

	if len(y) != 0 {
		return y
	}

	return x
}

func childMap(c Provider, m map[string]interface{}, n int, r *http.Request) map[string]interface{} {
	z := make(map[string]interface{}, len(m))

	for k, v := range m {
		if c.Tuner().IsExcluded(k) {
			continue
		}

		switch t := v.(type) {
		case []interface{}:
			z[k] = childSlice(c, t, n, r)
		case string:
			q, err := loadMap(c, t, n, r)
			if err != nil {
				continue
			}

			z[k] = q
		}
	}

	return z
}

func loadMap(c Provider, s string, n int, r *http.Request) (map[string]interface{}, error) {
	b, err := c.ReadFetch(s, r)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})

	if err := c.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	return expand(c, m, n, r).(map[string]interface{}), nil
}

func childSlice(c Provider, q []interface{}, n int, r *http.Request) []map[string]interface{} {
	ss := make([]string, len(q))

	for i, s := range q {
		switch v := s.(type) {
		case string:
			ss[i] = v
		}
	}

	vv := []map[string]interface{}{}
	mb, _ := c.ReadFetchMulti(ss, r)

	for i := range ss {
		m := make(map[string]interface{})
		s := c.Normalize(ss[i])

		if err := c.Unmarshal(mb[s], &m); err != nil {
			continue
		}

		if z := expand(c, m, n, r); z != nil {
			vv = append(vv, z.(map[string]interface{}))
		}
	}

	return vv
}
