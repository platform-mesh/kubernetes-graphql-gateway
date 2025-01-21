package apischema

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	v3JSONPath  = "./testdata/v3JSON.json"
	testDataDir = "./testdata"
)

// TODO: refactor
func TestConvertJSON(t *testing.T) {
	v3JSON, inErr := os.ReadFile(v3JSONPath)
	assert.NoError(t, inErr)
	assert.NotNil(t, v3JSON)
	v2JSON, cErr := ConvertJSON(v3JSON)
	assert.NoError(t, cErr)
	assert.NotNil(t, v2JSON)
	wErr := os.WriteFile(path.Join(testDataDir, "v3JSON_v2_out.json"), v2JSON, os.ModePerm)
	assert.NoError(t, wErr)
}
