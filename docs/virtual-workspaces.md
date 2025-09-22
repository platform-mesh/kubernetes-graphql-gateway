# Virtual Workspaces

## Configuration

Virtual workspaces are configured through a YAML configuration file that is mounted to the listener. The path to this file is specified using the `virtual-workspaces-config-path` configuration option.

### Configuration File Format

```yaml
virtualWorkspaces:
- name: example
  url: https://192.168.1.118:6443/services/apiexport/root/configmaps-view
  kubeconfig: PATH_TO_KCP_KUBECONFIG
  targetWorkspace: root:orgs:default  # Explicit full workspace path
- name: production-service
  url: https://your-kcp-server:6443/services/apiexport/root/your-export
  kubeconfig: PATH_TO_KCP_KUBECONFIG
  targetWorkspace: root:orgs:production  # Different organization
- name: team-service
  url: https://your-kcp-server:6443/services/apiexport/root/team-export
  kubeconfig: PATH_TO_KCP_KUBECONFIG
  targetWorkspace: root:orgs:team-a  # Team-specific organization
- name: flexible-service
  url: https://your-kcp-server:6443/services/apiexport/root/flexible-export
  kubeconfig: PATH_TO_KCP_KUBECONFIG
  # targetWorkspace not specified - will dynamically use default configuration
  # If gateway-url-default-kcp-workspace="production", this will resolve to "root:orgs:production"
  # If gateway-url-default-kcp-workspace="root:orgs:staging", this will use "root:orgs:staging" as-is
```

### Configuration Options

- `virtualWorkspaces`: Array of virtual workspace definitions
  - `name`: Unique identifier for the virtual workspace (used in URL paths)
  - `url`: Full URL to the virtual workspace or API export
  - `kubeconfig`: path to kcp kubeconfig
  - `targetWorkspace`: Optional target workspace path (e.g., "root:orgs:default", "root:orgs:production", "root:orgs:team-a")
    - If not specified, uses the `gateway-url-default-kcp-workspace` and `gateway-url-kcp-workspace-pattern` configuration values
    - If the default workspace contains ":", it's used as-is (e.g., "root:orgs:production")
    - If the default workspace is just an organization name, it's inserted into the workspace pattern
    - Default pattern: "root:orgs:{org}" where {org} is replaced with the organization name
    - Allows different virtual workspaces to target different organizations or workspace hierarchies dynamically

## Configuration Options

### Global Configuration

The following environment variables or configuration options control the default workspace resolution:

- `GATEWAY_URL_DEFAULT_KCP_WORKSPACE` (default: "default"): The default organization name
- `GATEWAY_URL_KCP_WORKSPACE_PATTERN` (default: "root:orgs:{org}"): The pattern for building workspace paths

### Environment Variables

Set the configuration path using:

```bash
export VIRTUAL_WORKSPACES_CONFIG_PATH="./bin/virtual-workspaces/config.yaml"
```

Set the default organization:

```bash
export GATEWAY_URL_DEFAULT_KCP_WORKSPACE="production"
```

Customize the workspace pattern (for different hierarchies):

```bash
export GATEWAY_URL_KCP_WORKSPACE_PATTERN="root:organizations:{org}"
# or for a flat structure:
export GATEWAY_URL_KCP_WORKSPACE_PATTERN="root:{org}"
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
2. **Schema Generation**: For each virtual workspace, the listener:
   - Creates a discovery client pointing to the virtual workspace URL
   - Generates OpenAPI schemas for the available resources
   - Stores the schema in a file at `virtual-workspace/{name}`
3. **Gateway Integration**: The gateway watches the schema files and exposes virtual workspaces as GraphQL endpoints
