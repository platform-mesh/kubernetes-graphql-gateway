package gateway_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const sleepTime = 2000 * time.Millisecond

type core struct {
	Pod       *podData `json:"Pod,omitempty"`
	CreatePod *podData `json:"createPod,omitempty"`
}

type metadata struct {
	Name      string
	Namespace string
}

type GraphQLResponse struct {
	Data   *graphQLData   `json:"data,omitempty"`
	Errors []graphQLError `json:"errors,omitempty"`
}

type graphQLData struct {
	Core                   *core                   `json:"core,omitempty"`
	CoreOpenmfpIO          *coreOpenmfpIo          `json:"core_openmfp_io,omitempty"`
	RbacAuthorizationK8sIo *RbacAuthorizationK8sIo `json:"rbac_authorization_k8s_io,omitempty"`
}

type graphQLError struct {
	Message   string                 `json:"message"`
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	Path      []interface{}          `json:"path,omitempty"`
}

type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func sendRequest(url, query string) (*GraphQLResponse, int, error) {
	reqBody := map[string]string{
		"query": query,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var bodyResp GraphQLResponse
	err = json.Unmarshal(respBytes, &bodyResp)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("response body is not json, but %s", respBytes)
	}

	return &bodyResp, resp.StatusCode, err
}

// writeToFile adds a new file to the watched directory which will trigger schema generation
func writeToFile(from, to string) error {
	specContent, err := os.ReadFile(from)
	if err != nil {
		return err
	}

	err = os.WriteFile(to, specContent, 0644)
	if err != nil {
		return err
	}

	// let's give some time to the manager to process the file and create a url
	time.Sleep(sleepTime)

	return nil
}
