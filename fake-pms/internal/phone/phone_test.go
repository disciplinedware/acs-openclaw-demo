package phone

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// The rules' own vectors: a form written with + normalizes as the rules
// normalize it; a form without + is refused, as a booking system refuses it.
func TestNormalizeAgreesWithTheRulesVectors(t *testing.T) {
	raw, err := os.ReadFile("../../../guardian/testdata/phones.yaml")
	require.NoError(t, err)
	var file struct {
		Vectors []struct {
			Name  string  `yaml:"name"`
			Input string  `yaml:"input"`
			E164  *string `yaml:"e164"`
		} `yaml:"vectors"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &file))
	require.NotEmpty(t, file.Vectors)
	for _, v := range file.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			got, ok := Normalize(v.Input)
			if v.E164 == nil || !strings.HasPrefix(v.Input, "+") {
				assert.False(t, ok, "got %q", got)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, *v.E164, got)
		})
	}
}
