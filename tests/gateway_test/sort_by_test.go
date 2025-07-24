package gateway_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openmfp/account-operator/api/v1alpha1"
)

// TestSortBy tests the sorting functionality of accounts by displayName
func (suite *CommonTestSuite) TestSortByListItems() {
	workspaceName := "myWorkspace"
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	require.NoError(suite.T(), suite.writeToFileWithClusterMetadata(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	suite.createAccountsForSorting(suite.T().Context())

	suite.T().Run("accounts_sorted_by_default", func(t *testing.T) {
		listResp, statusCode, err := suite.sendAuthenticatedRequest(url, listAccountsQuery(false))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, statusCode, "Expected status code 200")
		require.Nil(t, listResp.Errors, "GraphQL errors: %v", listResp.Errors)

		accounts := listResp.Data.CoreOpenmfpOrg.Accounts
		require.Len(t, accounts, 4, "Expected 4 accounts")

		expectedOrder := []string{"account-a", "account-b", "account-c", "account-d"}
		for i, oneAccount := range accounts {
			displayName := oneAccount.Metadata.Name
			require.Equal(t, expectedOrder[i], displayName,
				"Account at position %d should have displayName %s, got %s",
				i, expectedOrder[i], displayName)
		}
	})

	// Test sorted case
	suite.T().Run("accounts_sorted_by_displayName", func(t *testing.T) {
		listResp, statusCode, err := suite.sendAuthenticatedRequest(url, listAccountsQuery(true))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, statusCode, "Expected status code 200")
		require.Nil(t, listResp.Errors, "GraphQL errors: %v", listResp.Errors)

		accounts := listResp.Data.CoreOpenmfpOrg.Accounts
		require.Len(t, accounts, 4, "Expected 4 accounts")

		expectedOrder := []string{"account-d", "account-c", "account-b", "account-a"}
		for i, oneAccount := range accounts {
			displayName := oneAccount.Metadata.Name
			require.Equal(t, expectedOrder[i], displayName,
				"Account at position %d should have displayName %s, got %s",
				i, expectedOrder[i], displayName)
		}
	})
}

func (suite *CommonTestSuite) TestSortBySubscription() {
	ctx, cancel := context.WithCancel(suite.T().Context())
	defer cancel()
	suite.createAccountsForSorting(ctx)

	c := graphql.Subscribe(graphql.Params{
		Context:       ctx,
		RequestString: SubscribeAccounts(true),
		Schema:        suite.graphqlSchema,
	})

	count := 0
	expectedMsgCount := 4
	for count < expectedMsgCount { // Process exactly 4 messages
		select {
		case rawRes, ok := <-c:
			if !ok {
				return
			}

			accountList := rawRes.Data.(map[string]interface{})["core_openmfp_org_accounts"].([]interface{})
			if len(accountList) == 4 { // we have 4 accounts in total
				require.Equal(suite.T(), "account-d", accountList[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"])
			}

			count++
		case <-ctx.Done():
			return
		}
	}
}

func (suite *CommonTestSuite) createAccountsForSorting(ctx context.Context) {
	accounts := map[string]string{ // map[name]displayName
		"account-a": "displayName-D",
		"account-b": "displayName-C",
		"account-c": "displayName-B",
		"account-d": "displayName-A",
	}

	for name, displayName := range accounts {
		err := suite.runtimeClient.Create(ctx, &v1alpha1.Account{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1alpha1.AccountSpec{
				Type:        v1alpha1.AccountTypeAccount,
				DisplayName: displayName,
			},
		})
		require.NoError(suite.T(), err)
	}
	time.Sleep(sleepTime)
}
