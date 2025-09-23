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
- name: contentconfigurations
  url: https://your-kcp-server:6443/services/contentconfigurations
  kubeconfig: PATH_TO_KCP_KUBECONFIG
  # No targetWorkspace specified - workspace is resolved dynamically from user request:
  # User request: /virtual-workspace/contentconfigurations/root:orgs:alpha/query
  # → Connects to: /services/contentconfigurations/clusters/root:orgs:alpha/api/v1/configmaps
  # User request: /virtual-workspace/contentconfigurations/root:orgs:beta/query  
  # → Connects to: /services/contentconfigurations/clusters/root:orgs:beta/api/v1/configmaps
```

### Configuration Options

- `virtualWorkspaces`: Array of virtual workspace definitions
  - `name`: Unique identifier for the virtual workspace (used in URL paths)
  - `url`: Full URL to the virtual workspace or API export
  - `kubeconfig`: path to kcp kubeconfig
  - `targetWorkspace`: **REMOVED** - No longer supported. Workspace is now resolved dynamically from user requests.
  - **Dynamic Resolution**: The workspace is extracted from the user's GraphQL request URL at runtime
  - **Request-based**: Each user request can target a different workspace by specifying it in the URL
  - **Example**: `/virtual-workspace/contentconfigurations/root:orgs:alpha/query` → targets `root:orgs:alpha`
  - **Flexible**: Different users can access different organizations through the same virtual workspace configuration

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
2. **Generic Schema Generation**: For each virtual workspace, the listener:
   - Creates a discovery client pointing to the virtual workspace URL with a default workspace
   - Generates generic OpenAPI schemas for the available resources
   - Stores the schema in a file at `virtual-workspace/{name}`
3. **Dynamic Workspace Resolution**: When a user makes a GraphQL request:
   - The gateway extracts the workspace from the URL (e.g., `root:orgs:alpha`)
   - The roundtripper modifies the backend request to include the specific workspace
   - Example: `/services/contentconfigurations/api/v1/configmaps` → `/services/contentconfigurations/clusters/root:orgs:alpha/api/v1/configmaps`
4. **Gateway Integration**: The gateway exposes virtual workspaces as GraphQL endpoints with dynamic workspace targeting
