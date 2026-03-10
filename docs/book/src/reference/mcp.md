# kubebuilder mcp

> **Experimental**: This command is read-only by default. The protocol, resource URIs, and prompt names may change in future releases.

`kubebuilder mcp` starts a [Model Context Protocol (MCP)][mcp-spec] server over stdio, allowing AI coding assistants (such as Claude, Cursor, or GitHub Copilot) to discover Kubebuilder project metadata and operator-development guidance directly from the binary.

## Usage

```bash
kubebuilder mcp [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--transport` | `stdio` | Transport to use. Only `stdio` is supported. |
| `--project-dir` | _(current directory)_ | Directory containing the `PROJECT` file. |

## How it works

The server speaks [JSON-RPC 2.0][jsonrpc] over standard input and output (one message per line). It implements the read-only subset of the MCP specification:

- **Resources** — structured read-only data describing the current project and binary.
- **Prompts** — curated, version-aware guidance templates for common operator-development tasks.

No mutating tools are exposed.

## Resources

| URI | Description |
|-----|-------------|
| `kubebuilder://version` | Kubebuilder version information (JSON) |
| `kubebuilder://project/config` | Parsed `PROJECT` file metadata (JSON) |
| `kubebuilder://project/apis` | API resources defined in the project (JSON) |
| `kubebuilder://project/plugins` | Active plugin chain (JSON) |

Project-scoped resources (`kubebuilder://project/*`) require the server to be started from—or pointed at—a valid Kubebuilder project directory. If no `PROJECT` file is found they return an actionable error message.

### Example: `kubebuilder://version`

```json
{"version":"v4.5.0"}
```

### Example: `kubebuilder://project/config`

```json
{
  "version": "3",
  "domain": "example.com",
  "repo": "github.com/example/myoperator",
  "projectName": "myoperator",
  "layout": ["go.kubebuilder.io/v4", "kustomize.common.kubebuilder.io/v2"],
  "multiGroup": false,
  "cliVersion": "4.5.0"
}
```

### Example: `kubebuilder://project/apis`

```json
[
  {
    "group": "apps",
    "version": "v1alpha1",
    "kind": "Foo",
    "plural": "foos",
    "controller": true,
    "hasAPI": true,
    "namespaced": true
  }
]
```

## Prompts

| Name | Description |
|------|-------------|
| `reconcile-best-practices` | Best practices for implementing a Kubernetes controller `Reconcile` function |
| `project-summary` | Narrative summary of the current Kubebuilder project |

Prompts are version-aware: they embed the running Kubebuilder version and, where available, the current project context.

## Connecting an AI assistant

Configure your AI client to run `kubebuilder mcp` as an MCP server. For example, with Claude Desktop, add the following to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "kubebuilder": {
      "command": "kubebuilder",
      "args": ["mcp"]
    }
  }
}
```

For projects not in the current working directory, use `--project-dir`:

```json
{
  "mcpServers": {
    "kubebuilder": {
      "command": "kubebuilder",
      "args": ["mcp", "--project-dir", "/path/to/my-operator"]
    }
  }
}
```

## Safety and limitations

- **Read-only by default.** No files are created or modified.
- **No shell execution.** The server calls Go functions directly; it never passes arbitrary strings to a shell.
- **stdio only.** No network listener is opened.
- **Experimental.** Resource URIs, prompt names, and payload shapes are subject to change between minor releases.

[mcp-spec]: https://spec.modelcontextprotocol.io/
[jsonrpc]: https://www.jsonrpc.org/specification
