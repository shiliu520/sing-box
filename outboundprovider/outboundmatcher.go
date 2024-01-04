package outboundprovider

import (
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type outboundMatcher interface {
	match(outbound *option.Outbound) bool
}

type outboundTagMatcher regexp.Regexp

func (m *outboundTagMatcher) match(outbound *option.Outbound) bool {
	return (*regexp.Regexp)(m).MatchString(outbound.Tag)
}

type outboundTypeMatcher string

func (m outboundTypeMatcher) match(outbound *option.Outbound) bool {
	return string(m) == outbound.Type
}

func newOutboundMatcher(s string) (outboundMatcher, error) {
	var ss string
	switch {
	case strings.HasPrefix(s, "type:"):
		ss = strings.TrimPrefix(s, "type:")
		return outboundTypeMatcher(ss), nil
	case strings.HasPrefix(s, "tag:"):
		ss = strings.TrimPrefix(s, "tag:")
		regex, err := regexp.Compile(ss)
		if err != nil {
			return nil, E.Cause(err, "invalid rule: ", s)
		}
		return (*outboundTagMatcher)(regex), nil
	default:
		ss = s
		regex, err := regexp.Compile(ss)
		if err != nil {
			return nil, E.Cause(err, "invalid rule: ", s)
		}
		return (*outboundTagMatcher)(regex), nil
	}
}

type outboundMatcherGroup struct {
	rules   []outboundMatcher
	logical string // and / or
}

func newOutboundMatcherGroup(rules []string, logical string) (outboundMatcher, error) {
	switch logical {
	case "and":
	case "or":
	case "":
		return nil, E.New("missing logical")
	default:
		return nil, E.New("invalid logical: ", logical)
	}
	if len(rules) == 0 {
		return nil, E.New("missing rules")
	}
	g := &outboundMatcherGroup{
		rules:   make([]outboundMatcher, len(rules)),
		logical: logical,
	}
	for i, rule := range rules {
		matcher, err := newOutboundMatcher(rule)
		if err != nil {
			return nil, E.Cause(err, "invalid rule[", i, "]: ", rule)
		}
		g.rules[i] = matcher
	}
	return g, nil
}

func (g *outboundMatcherGroup) matchAnd(outbound *option.Outbound) bool {
	for _, rule := range g.rules {
		if !rule.match(outbound) {
			return false
		}
	}
	return true
}

func (g *outboundMatcherGroup) matchOr(outbound *option.Outbound) bool {
	for _, rule := range g.rules {
		if rule.match(outbound) {
			return true
		}
	}
	return false
}

func (g *outboundMatcherGroup) match(outbound *option.Outbound) bool {
	switch g.logical {
	case "and":
		return g.matchAnd(outbound)
	case "or":
		return g.matchOr(outbound)
	}
	panic("unreachable")
}
