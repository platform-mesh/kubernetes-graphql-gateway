package gateway_test

type coreOpenmfpOrg struct {
	Account       *account   `json:"Account,omitempty"`
	Accounts      []*account `json:"Accounts,omitempty"`
	CreateAccount *account   `json:"createAccount,omitempty"`
	DeleteAccount *bool      `json:"deleteAccount,omitempty"`
}

type account struct {
	Metadata metadata    `json:"metadata"`
	Spec     accountSpec `json:"spec"`
}

type accountSpec struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

func createAccountMutation() string {
	return `
mutation {
  core_openmfp_org {
    createAccount(
      object:  {
        metadata: {
          name: "test-account"
        },
        spec: {
          type: "account",
          displayName:"test-account-display-name"
        }
      }
    ){
      metadata {
        name
      }
      spec {
        type,
        displayName
      }
    }
  }
}
    `
}

func getAccountQuery() string {
	return `
        query {
			core_openmfp_org {
			Account(name: "test-account") {
			  metadata {
				name
			  }
			  spec {
				type,
				displayName
			  }
			}
			}
		}
    `
}

func listAccountsQuery(sortByDisplayName bool) string {
	if sortByDisplayName {
		return `query {
			core_openmfp_org {
			Accounts(sortBy: "spec.displayName") {
			  metadata {
				name
			  }
			  spec {
				type,
				displayName
			  }}}}`
	}

	return `query {
			core_openmfp_org {
			Accounts {
			  metadata {
				name
			  }
			  spec {
				type,
				displayName
			  }}}}`
}

func deleteAccountMutation() string {
	return `
		mutation {
		  core_openmfp_org {
			deleteAccount(name: "test-account")
		  }
		}
    `
}

func SubscribeAccounts(sortByDisplayName bool) string {
	if sortByDisplayName {
		return `subscription {
				core_openmfp_org_accounts(sortBy: "spec.displayName") {
					metadata { name }
					spec { displayName }
				}
			}
		`
	}
	return `subscription {
			core_openmfp_org_accounts {
				metadata { name }
				spec { displayName }
			}
		}
	`
}
