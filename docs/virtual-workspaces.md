# Virtual Workspaces

## Configuration

Virtual workspaces are configured through a YAML configuration file that is mounted to the listener. The path to this file is specified using the `virtual-workspaces-config-path` configuration option.

### Configuration File Format

```yaml
virtualWorkspaces:
- name: example
  url: https://192.168.1.118:6443/services/apiexport/root/configmaps-view
  kubeconfig: PATH_TO_KCP_KUBECONFIG
- name: another-service
  url: https://your-kcp-server:6443/services/apiexport/root/your-export
  kubeconfig: PATH_TO_KCP_KUBECONFIG
```

### Configuration Options

- `virtualWorkspaces`: Array of virtual workspace definitions
  - `name`: Unique identifier for the virtual workspace (used in URL paths)
  - `url`: Full URL to the virtual workspace or API export
  - `kubeconfig`: path to kcp kubeconfig

## Environment Variables

Set the configuration path using:

```bash
export VIRTUAL_WORKSPACES_CONFIG_PATH="./bin/virtual-workspaces/config.yaml"
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
