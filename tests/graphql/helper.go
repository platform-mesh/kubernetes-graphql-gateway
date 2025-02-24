package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type CoreData struct {
	Pod       *PodData `json:"Pod,omitempty"`
	CreatePod *PodData `json:"createPod,omitempty"`

	Service *ServiceData `json:"Service,omitempty"`

	Account       *AccountData `json:"Account,omitempty"`
	CreateAccount *AccountData `json:"createAccount,omitempty"`
	DeleteAccount *bool        `json:"deleteAccount,omitempty"`
}

type Metadata struct {
	Name      string
	Namespace string
}

type GraphQLResponse struct {
	Data   *GraphQLData   `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLData struct {
	Core          *CoreData `json:"core,omitempty"`
	CoreOpenmfpIO *CoreData `json:"core_openmfp_io,omitempty"`
}

type GraphQLError struct {
	Message   string                 `json:"message"`
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	Path      []interface{}          `json:"path,omitempty"`
}

type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func SendRequest(url, query string) (*GraphQLResponse, int, error) {
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
	v := resp.Body
	fmt.Println(v)

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
