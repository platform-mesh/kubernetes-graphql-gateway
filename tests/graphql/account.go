package graphql

// AccountData represents the Account resource with its metadata and specification.
type AccountData struct {
	Metadata Metadata    `json:"metadata"`
	Spec     AccountSpec `json:"spec"`
}

// AccountSpec defines the desired state of the Account.
type AccountSpec struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

func CreateAccountMutation() string {
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

func GetAccountQuery() string {
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

func DeleteAccountMutation() string {
	return `
		mutation {
		  core_openmfp_io {
			deleteAccount(namespace: "default", name: "test-account")
		  }
		}
    `
}
