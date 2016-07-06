package main

import (
	"bytes"
	"hash/fnv"

	"github.com/buger/gor/proto"
)

type HTTPModifier struct {
	config *HTTPModifierConfig
}

func NewHTTPModifier(config *HTTPModifierConfig) *HTTPModifier {
	// Optimization to skip modifier completely if we do not need it
	if len(config.urlRegexp) == 0 &&
		len(config.urlNegativeRegexp) == 0 &&
		len(config.urlRewrite) == 0 &&
		len(config.headerFilters) == 0 &&
		len(config.headerNegativeFilters) == 0 &&
		len(config.headerHashFilters) == 0 &&
		len(config.paramHashFilters) == 0 &&
		len(config.params) == 0 &&
		len(config.headers) == 0 &&
		len(config.paramFilters) == 0 &&
		len(config.methods) == 0 {
		return nil
	}

	return &HTTPModifier{config: config}
}

func (m *HTTPModifier) Rewrite(payload []byte) (response []byte) {
	if !proto.IsHTTPPayload(payload) {
		return payload
	}

	if len(m.config.methods) > 0 {
		method := proto.Method(payload)

		matched := false

		for _, m := range m.config.methods {
			if bytes.Equal(method, m) {
				matched = true
				break
			}
		}

		if !matched {
			return
		}
	}

	if len(m.config.headers) > 0 {
		for _, header := range m.config.headers {
			payload = proto.SetHeader(payload, []byte(header.Name), []byte(header.Value))
		}
	}

	if len(m.config.params) > 0 {
		for _, param := range m.config.params {
			payload = proto.SetPathParam(payload, param.Name, param.Value)
		}
	}

	if len(m.config.urlRegexp) > 0 {
		path := proto.Path(payload)

		matched := false

		for _, f := range m.config.urlRegexp {
			if f.regexp.Match(path) {
				matched = true
				break
			}
		}

		if !matched {
			return
		}
	}

	if len(m.config.urlNegativeRegexp) > 0 {
		path := proto.Path(payload)

		for _, f := range m.config.urlNegativeRegexp {
			if f.regexp.Match(path) {
				return
			}
		}
	}

	if len(m.config.headerFilters) > 0 {
		for _, f := range m.config.headerFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 && !f.regexp.Match(value) {
				return
			}
		}
	}

	if len(m.config.headerNegativeFilters) > 0 {
		for _, f := range m.config.headerNegativeFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 && f.regexp.Match(value) {
				return
			}
		}
	}

	if len(m.config.headerHashFilters) > 0 {
		for _, f := range m.config.headerHashFilters {
			value := proto.Header(payload, f.name)

			if len(value) > 0 {
				hasher := fnv.New32a()
				hasher.Write(value)

				if (hasher.Sum32() % 100) >= f.percent {
					return
				}
			}
		}
	}

	if len(m.config.paramHashFilters) > 0 {
		for _, f := range m.config.paramHashFilters {
			value, s, _ := proto.PathParam(payload, f.name)

			if s != -1 {
				hasher := fnv.New32a()
				hasher.Write(value)

				if (hasher.Sum32() % 100) >= f.percent {
					return
				}
			}
		}
	}

	if len(m.config.urlRewrite) > 0 {
		path := proto.Path(payload)

		for _, f := range m.config.urlRewrite {
			if f.src.Match(path) {
				path = f.src.ReplaceAll(path, f.target)
				payload = proto.SetPath(payload, path)

				break
			}
		}
	}

	if len(m.config.paramFilters) > 0 {
		matched := false

		params := proto.Params(payload)
		if len(params) == 0 {
			return
		}

	outer:
		for pn, pvs := range params {
			for _, pv := range pvs {
				for _, pf := range m.config.paramFilters {
					if bytes.Equal(pf.Name, []byte(pn)) && bytes.Equal(pf.Value, []byte(pv)) {
						matched = true
						break outer
					}
				}
			}
		}

		if !matched {
			return
		}
	}

	return payload
}
