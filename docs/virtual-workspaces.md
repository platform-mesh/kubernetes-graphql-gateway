# Virtual Workspaces

## Configuration

Virtual workspaces are configured through a YAML configuration file that is mounted to the listener. The path to this file is specified using the `virtual-workspaces-config-path` configuration option.

### Configuration File Format

```yaml
virtualWorkspaces:
- name: example
  url: https://192.168.1.118:6443/services/apiexport/root/configmaps-view
  kubeconfig: PATH_TO_KCP_KUBECONFIG
  # Workspace is resolved dynamically from user request:
  # User request: /virtual-workspace/example/root:orgs:alpha/query
  # → Connects to: /services/apiexport/root/configmaps-view/clusters/root:orgs:alpha/api/v1/configmaps
```

### Configuration Options

- `virtualWorkspaces`: Array of virtual workspace definitions
  - `name`: Unique identifier for the virtual workspace (used in URL paths)
  - `url`: Full URL to the virtual workspace or API export
  - `kubeconfig`: Path to KCP kubeconfig

### Dynamic Workspace Resolution

Virtual workspaces use **dynamic workspace resolution**:
- Workspace is extracted from the GraphQL request URL at runtime
- Each request can target different workspaces: `/virtual-workspace/contentconfigurations/root:orgs:alpha/query`
- No need to predefine target workspaces in configuration

## Environment Variables

```bash
# Virtual workspaces configuration file path
export VIRTUAL_WORKSPACES_CONFIG_PATH="./config/virtual-workspaces.yaml"

# Default workspace for schema generation (default: "root")  
export GATEWAY_URL_DEFAULT_KCP_WORKSPACE="root"

# Workspace pattern for building full paths (default: "root:orgs:{org}")
export GATEWAY_URL_KCP_WORKSPACE_PATTERN="root:orgs:{org}"
```

## URL Pattern

Virtual workspaces are accessible through the gateway using the following URL pattern:

```
/kubernetes-graphql-gateway/virtual-workspace/{VIRTUAL_WS_NAME}/{KCP_CLUSTER_NAME}/query
```

For example:
- Normal workspace: `/kubernetes-graphql-gateway/root:abc:abc/query`
- Virtual workspace: `/kubernetes-graphql-gateway/virtualworkspace/example/root:abc:abc/query`

## How It Works

1. **Configuration Watching**: The listener watches the virtual workspaces configuration file for changes
2. **Generic Schema Generation**: For each virtual workspace, the listener:
   - Creates a discovery client pointing to the virtual workspace URL with a default workspace
   - Generates generic OpenAPI schemas for the available resources
   - Stores the schema in a file at `virtual-workspace/{name}`
3. **Dynamic Workspace Resolution**: When a user makes a GraphQL request:
   - The gateway extracts the workspace from the URL (e.g., `root:orgs:alpha`)
   - The roundtripper modifies the backend request to include the specific workspace
   - Example: `/services/contentconfigurations/api/v1/configmaps` → `/services/contentconfigurations/clusters/root:orgs:alpha/api/v1/configmaps`
4. **Gateway Integration**: The gateway exposes virtual workspaces as GraphQL endpoints with dynamic workspace targeting
