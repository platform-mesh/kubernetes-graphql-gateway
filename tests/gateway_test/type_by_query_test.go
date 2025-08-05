package gateway_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openmfp/golang-commons/logger/testlogger"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
)

func getGateway() (*schema.Gateway, error) {
	// Read the schema file and extract definitions
	definitions, err := readDefinitionFromFile("./testdata/kubernetes")
	if err != nil {
		return nil, err
	}

	return schema.New(testlogger.New().Logger, definitions, resolver.New(testlogger.New().Logger, nil))
}

// readDefinitionFromFile reads OpenAPI definitions from a schema file
func readDefinitionFromFile(filename string) (spec.Definitions, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var schemaData map[string]interface{}
	if err := json.NewDecoder(file).Decode(&schemaData); err != nil {
		return nil, err
	}

	var definitions spec.Definitions
	if defsRaw, exists := schemaData["definitions"]; exists {
		defsBytes, err := json.Marshal(defsRaw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(defsBytes, &definitions); err != nil {
			return nil, err
		}
	}

	return definitions, nil
}

func TestTypeByCategory(t *testing.T) {
	g, err := getGateway()
	require.NoError(t, err)

	res := graphql.Do(graphql.Params{
		Context:       t.Context(),
		Schema:        *g.GetSchema(),
		RequestString: typeByCategoryQuery(),
	})

	require.Nil(t, res.Errors)
	require.NotNil(t, res.Data)

	data := res.Data.(map[string]interface{})
	typeByCategory := data["typeByCategory"].([]interface{})
	firstItem := typeByCategory[0].(map[string]interface{})

	assert.Equal(t, "networking_istio_io", firstItem["group"])
}

func typeByCategoryQuery() string {
	return `
		query{
		  typeByCategory(name: "istio-io"){
			group
			version
			kind
			scope
		  }
		}`
}
