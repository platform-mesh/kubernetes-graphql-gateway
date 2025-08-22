package gateway_test

type corePlatformMeshIo struct {
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
  core_platform_mesh_io {
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
			core_platform_mesh_io {
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
			core_platform_mesh_io {
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
			core_platform_mesh_io {
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
		  core_platform_mesh_io {
			deleteAccount(name: "test-account")
		  }
		}
    `
}

func SubscribeAccounts(sortByDisplayName bool) string {
	if sortByDisplayName {
		return `subscription {
				core_platform_mesh_io_accounts(sortBy: "spec.displayName") {
					metadata { name }
					spec { displayName }
				}
			}
		`
	}
	return `subscription {
			core_platform_mesh_io_accounts {
				metadata { name }
				spec { displayName }
			}
		}
	`
}
