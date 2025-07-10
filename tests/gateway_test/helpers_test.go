package gateway_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const sleepTime = 500 * time.Millisecond

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
	CoreOpenmfpOrg         *coreOpenmfpOrg         `json:"core_openmfp_org,omitempty"`
	RbacAuthorizationK8sIO *RbacAuthorizationK8sIO `json:"rbac_authorization_k8s_io,omitempty"`
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
	return sendRequestWithAuth(url, query, "")
}

func sendRequestWithAuth(url, query, token string) (*GraphQLResponse, int, error) {
	reqBody := map[string]string{
		"query": query,
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Add Authorization header if token is provided
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
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
