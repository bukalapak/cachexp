// Package cachexp provides cache expansion mechanism using selected Provider.
package cachexp

import (
	"net/http"

	multierror "github.com/hashicorp/go-multierror"
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
		z, err := expand(c, m, c.Tuner().ExpandDepth(), r)
		mrr := multierror.Append(err)

		d, err := c.Marshal(z)
		return d, multierror.Append(mrr, err).ErrorOrNil()
	}

	return b, nil
}

func expand(c Provider, m map[string]interface{}, n int, r *http.Request) (interface{}, error) {
	d := n
	y := []map[string]interface{}{}
	x := make(map[string]interface{})

	var asSlice bool
	var mrr *multierror.Error

	for k, v := range m {
		if k == c.Tuner().ExpandKey() {
			d--

			if d >= 0 {
				switch t := v.(type) {
				case map[string]interface{}:
					vx, err := childMap(c, t, d, r)
					mrr = multierror.Append(mrr, err)

					for kk, vv := range vx {
						x[kk] = vv
					}
				case []interface{}:
					yx, err := childSlice(c, t, d, r)
					mrr = multierror.Append(mrr, err)

					y = append(y, yx...)
					asSlice = true
				}
			}

			continue
		}

		switch t := v.(type) {
		case map[string]interface{}:
			var err error

			x[k], err = expand(c, t, d, r)
			mrr = multierror.Append(mrr, err)
		default:
			x[k] = v
		}
	}

	if len(y) != 0 && len(x) != 0 {
		x[c.Tuner().PlaceholderKey()] = y
		return x, mrr.ErrorOrNil()
	}

	if asSlice {
		return y, mrr.ErrorOrNil()
	}

	return x, mrr.ErrorOrNil()
}

func childMap(c Provider, m map[string]interface{}, n int, r *http.Request) (map[string]interface{}, error) {
	z := make(map[string]interface{}, len(m))

	var mrr *multierror.Error

	for k, v := range m {
		if c.Tuner().IsExcluded(k) {
			continue
		}

		switch t := v.(type) {
		case []interface{}:
			var err error

			z[k], err = childSlice(c, t, n, r)
			mrr = multierror.Append(mrr, err)
		case string:
			q, err := loadMap(c, t, n, r)
			if err != nil {
				continue
			}

			z[k] = q
		}
	}

	return z, mrr.ErrorOrNil()
}

func loadMap(c Provider, s string, n int, r *http.Request) (map[string]interface{}, error) {
	b, err := c.ReadFetch(s, r)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})

	if err = c.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	z, err := expand(c, m, n, r)
	return z.(map[string]interface{}), err
}

func childSlice(c Provider, q []interface{}, n int, r *http.Request) ([]map[string]interface{}, error) {
	ss := make([]string, len(q))

	for i, s := range q {
		switch v := s.(type) {
		case string:
			ss[i] = v
		}
	}

	vv := []map[string]interface{}{}

	mb, err := c.ReadFetchMulti(ss, r)
	mrr := multierror.Append(err)

	for i := range ss {
		m := make(map[string]interface{})
		s := c.Normalize(ss[i])

		if err = c.Unmarshal(mb[s], &m); err != nil {
			continue
		}

		z, err := expand(c, m, n, r)
		mrr = multierror.Append(mrr, err)

		if z != nil {
			vv = append(vv, z.(map[string]interface{}))
		}
	}

	return vv, mrr.ErrorOrNil()
}
