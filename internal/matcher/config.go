package matcher

import (
	"fmt"
	"os"
	"regexp"
)

// ConfigDef is the YAML structure for a user-defined matcher.
type ConfigDef struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
}

// LoadFromConfig compiles user-defined matchers from config.
func LoadFromConfig(configs []ConfigDef) []Matcher {
	var matchers []Matcher
	for _, cfg := range configs {
		pat, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "linkify: bad matcher pattern %q (%s): %v\n", cfg.Name, cfg.Pattern, err)
			continue
		}
		matchers = append(matchers, &RegexMatcher{
			MatcherName: cfg.Name,
			Pattern:     pat,
		})
	}
	return matchers
}
