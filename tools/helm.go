package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// HelmInstallTool returns the MCP tool definition for installing Helm charts
func HelmInstallTool() mcp.Tool {
	return mcp.NewTool("helmInstall",
		mcp.WithDescription("Install a Helm chart to the Kubernetes cluster"),
		mcp.WithString("releaseName", mcp.Required(), mcp.Description("Name of the Helm release")),
		mcp.WithString("chartName", mcp.Required(), mcp.Description("Name or path of the Helm chart")),
		mcp.WithString("namespace", mcp.Description("Kubernetes namespace for the release")),
		mcp.WithString("repoURL", mcp.Description("Helm repository URL (optional)")),
		mcp.WithObject("values", mcp.Description("Values to override in the chart")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Helm Install",
			DestructiveHint: mcp.ToBoolPtr(true),
		}),
	)
}

// HelmUpgradeTool returns the MCP tool definition for upgrading Helm releases
func HelmUpgradeTool() mcp.Tool {
	return mcp.NewTool("helmUpgrade",
		mcp.WithDescription("Upgrade an existing Helm release"),
		mcp.WithString("releaseName", mcp.Required(), mcp.Description("Name of the Helm release to upgrade")),
		mcp.WithString("chartName", mcp.Required(), mcp.Description("Name or path of the Helm chart")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace of the release")),
		mcp.WithObject("values", mcp.Required(), mcp.Description("Values to override in the chart")),
		mcp.WithObject("repoURL", mcp.Required(), mcp.Description("URL of the Helm repository")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Helm Upgrade",
			DestructiveHint: mcp.ToBoolPtr(true),
		}),
	)
}

// HelmUninstallTool returns the MCP tool definition for uninstalling Helm releases
func HelmUninstallTool() mcp.Tool {
	return mcp.NewTool("helmUninstall",
		mcp.WithDescription("Uninstall a Helm release from the Kubernetes cluster"),
		mcp.WithString("releaseName", mcp.Required(), mcp.Description("Name of the Helm release to uninstall")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace of the release")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Helm Uninstall",
			DestructiveHint: mcp.ToBoolPtr(true),
		}),
	)
}

// HelmListTool returns the MCP tool definition for listing Helm releases
func HelmListTool() mcp.Tool {
	return mcp.NewTool("helmList",
		mcp.WithDescription("List all Helm releases in the cluster or a specific namespace"),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace to list releases from (empty for all namespaces)")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Helm List",
			ReadOnlyHint: mcp.ToBoolPtr(true),
		}),
	)
}

// HelmGetTool returns the MCP tool definition for getting Helm release details
func HelmGetTool() mcp.Tool {
	return mcp.NewTool("helmGet",
		mcp.WithDescription("Get details of a specific Helm release"),
		mcp.WithString("releaseName", mcp.Required(), mcp.Description("Name of the Helm release")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace of the release")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Helm Get",
			ReadOnlyHint: mcp.ToBoolPtr(true),
		}),
	)
}

// HelmHistoryTool returns the MCP tool definition for getting Helm release history
func HelmHistoryTool() mcp.Tool {
	return mcp.NewTool("helmHistory",
		mcp.WithDescription("Get the history of a Helm release"),
		mcp.WithString("releaseName", mcp.Required(), mcp.Description("Name of the Helm release")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace of the release")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Helm History",
			ReadOnlyHint: mcp.ToBoolPtr(true),
		}),
	)
}

// HelmRollbackTool returns the MCP tool definition for rolling back Helm releases
func HelmRollbackTool() mcp.Tool {
	return mcp.NewTool("helmRollback",
		mcp.WithDescription("Rollback a Helm release to a previous revision"),
		mcp.WithString("releaseName", mcp.Required(), mcp.Description("Name of the Helm release to rollback")),
		mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace of the release")),
		mcp.WithNumber("revision", mcp.Required(), mcp.Description("Revision number to rollback to (0 for previous)")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Helm Rollback",
			DestructiveHint: mcp.ToBoolPtr(true),
		}),
	)
}

func HelmRepoAddTool() mcp.Tool {
	return mcp.NewTool("helmRepoAdd",
		mcp.WithDescription("Add a Helm repository"),
		mcp.WithString("repoName", mcp.Required(), mcp.Description("Name of the Helm repository")),
		mcp.WithString("repoURL", mcp.Required(), mcp.Description("URL of the Helm repository")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Helm Repo Add",
			DestructiveHint: mcp.ToBoolPtr(true),
		}),
	)
}

func HelmRepoListTool() mcp.Tool {
	return mcp.NewTool("helmRepoList",
		mcp.WithDescription("List all Helm repositories"),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Helm Repo List",
			ReadOnlyHint: mcp.ToBoolPtr(true),
		}),
	)
}
