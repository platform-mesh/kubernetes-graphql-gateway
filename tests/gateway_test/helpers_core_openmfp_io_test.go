package gateway_test

type coreOpenmfpIo struct {
	Account       *account `json:"Account,omitempty"`
	CreateAccount *account `json:"createAccount,omitempty"`
	DeleteAccount *bool    `json:"deleteAccount,omitempty"`
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
  core_openmfp_io {
    createAccount(
      namespace: "default", 
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
			core_openmfp_io {
			Account(namespace: "default", name: "test-account") {
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

func deleteAccountMutation() string {
	return `
		mutation {
		  core_openmfp_io {
			deleteAccount(namespace: "default", name: "test-account")
		  }
		}
    `
}
