package graphql

import (
	"bufio"
	"bytes"
	"context"
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

// SubscribeToGraphQL sends an SSE request and streams data to a channel
func SubscribeToGraphQL(url, query string) (chan map[string]interface{}, func(), error) {
	ctx, cancelFn := context.WithCancel(context.Background())

	payload := map[string]string{"query": query}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, cancelFn, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, cancelFn, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	msgChan := make(chan map[string]interface{})
	go func(msgChan chan map[string]interface{}) {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Failed to send request: %v", err)
			return
		}

		defer resp.Body.Close() // closes SSE stream

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Unexpected status code: %d", resp.StatusCode)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				if len(line) == 0 || line == ":" {
					continue // Skip heartbeat or empty lines
				}

				// Handle SSE message lines starting with "data: "
				if len(line) > 6 && line[:6] == "data: " {
					rawData := line[6:] // Extract data after "data: "
					var data map[string]interface{}
					err := json.Unmarshal([]byte(rawData), &data)
					if err != nil {
						fmt.Printf("Failed to parse message: %v", err)
						continue
					}
					msgChan <- data
				}
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading SSE stream: %v", err)
		}
	}(msgChan)

	return msgChan, cancelFn, nil
}
