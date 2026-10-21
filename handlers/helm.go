package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/reza-gholizade/k8s-mcp-server/pkg/helm"
)

// HelmInstall returns a handler function for the helmInstall tool

func HelmInstall(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		releaseName, err := getRequiredStringArg(args, "releaseName")
		if err != nil {
			return nil, err
		}

		chartName, err := getRequiredStringArg(args, "chartName")
		if err != nil {
			return nil, err
		}

		namespace := getStringArg(args, "namespace", "default")
		repoURL := getStringArg(args, "repoURL", "")

		values := make(map[string]interface{})
		if v, exists := args["values"]; exists {
			if valuesMap, ok := v.(map[string]interface{}); ok {
				values = valuesMap
			}
		}

		release, err := client.InstallChart(ctx, namespace, releaseName, chartName, repoURL, values)
		if err != nil {
			return nil, fmt.Errorf("failed to install chart: %w", err)
		}

		jsonResponse, err := json.Marshal(release)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmUpgrade returns a handler function for the helmUpgrade tool
func HelmUpgrade(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		releaseName, err := getRequiredStringArg(args, "releaseName")
		if err != nil {
			return nil, err
		}

		chartName, err := getRequiredStringArg(args, "chartName")
		if err != nil {
			return nil, err
		}

		namespace := getStringArg(args, "namespace", "default")

		values := make(map[string]interface{})
		if v, exists := args["values"]; exists {
			if valuesMap, ok := v.(map[string]interface{}); ok {
				values = valuesMap
			}
		}

		release, err := client.UpgradeChart(ctx, namespace, releaseName, chartName, values)
		if err != nil {
			return nil, fmt.Errorf("failed to upgrade chart: %w", err)
		}

		jsonResponse, err := json.Marshal(release)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmUninstall returns a handler function for the helmUninstall tool
func HelmUninstall(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		releaseName, err := getRequiredStringArg(args, "releaseName")
		if err != nil {
			return nil, err
		}

		namespace := getStringArg(args, "namespace", "default")

		err = client.UninstallChart(ctx, namespace, releaseName)
		if err != nil {
			return nil, fmt.Errorf("failed to uninstall chart: %w", err)
		}

		response := map[string]string{
			"status":  "success",
			"message": fmt.Sprintf("Successfully uninstalled release '%s' from namespace '%s'", releaseName, namespace),
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmList returns a handler function for the helmList tool
func HelmList(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		namespace := getStringArg(args, "namespace", "")

		releases, err := client.ListReleases(ctx, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to list releases: %w", err)
		}

		jsonResponse, err := json.Marshal(releases)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmGet returns a handler function for the helmGet tool
func HelmGet(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		releaseName, err := getRequiredStringArg(args, "releaseName")
		if err != nil {
			return nil, err
		}

		namespace := getStringArg(args, "namespace", "default")

		release, err := client.GetRelease(ctx, namespace, releaseName)
		if err != nil {
			return nil, fmt.Errorf("failed to get release: %w", err)
		}

		jsonResponse, err := json.Marshal(release)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmHistory returns a handler function for the helmHistory tool
func HelmHistory(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		releaseName, err := getRequiredStringArg(args, "releaseName")
		if err != nil {
			return nil, err
		}

		namespace := getStringArg(args, "namespace", "default")

		history, err := client.GetReleaseHistory(ctx, namespace, releaseName)
		if err != nil {
			return nil, fmt.Errorf("failed to get release history: %w", err)
		}

		jsonResponse, err := json.Marshal(history)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmRollback returns a handler function for the helmRollback tool
func HelmRollback(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		releaseName, err := getRequiredStringArg(args, "releaseName")
		if err != nil {
			return nil, err
		}

		namespace := getStringArg(args, "namespace", "default")

		revision, err := parseHelmRevision(args)
		if err != nil {
			return nil, err
		}

		err = client.RollbackRelease(ctx, namespace, releaseName, revision)
		if err != nil {
			return nil, fmt.Errorf("failed to rollback release: %w", err)
		}

		response := map[string]interface{}{
			"status":   "success",
			"message":  fmt.Sprintf("Successfully rolled back release '%s' in namespace '%s'", releaseName, namespace),
			"revision": revision,
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// HelmRepoAdd returns a handler function for the helmRepoAdd tool
func HelmRepoAdd(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid arguments type: expected map[string]interface{}")
		}

		repoName, err := getRequiredStringArg(args, "repoName")
		if err != nil {
			return nil, err
		}

		repoURL, err := getRequiredStringArg(args, "repoURL")
		if err != nil {
			return nil, err
		}

		err = client.HelmRepoAdd(ctx, repoName, repoURL)
		if err != nil {
			return nil, fmt.Errorf("failed to add repository: %w", err)
		}

		response := map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Successfully added repository '%s' with URL '%s'", repoName, repoURL),
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

func HelmRepoList(client *helm.Client) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repos, err := client.HelmRepoList(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		jsonResponse, err := json.Marshal(repos)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize response: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}
}

// parseHelmRevision accepts the numeric JSON representations used by MCP clients.
// Revision 0 has Helm's documented meaning of "previous revision".
func parseHelmRevision(args map[string]interface{}) (int, error) {
	rawRevision, exists := args["revision"]
	if !exists || rawRevision == nil {
		return 0, fmt.Errorf("missing required parameter: revision")
	}

	invalid := func(value interface{}) (int, error) {
		return 0, fmt.Errorf("invalid revision %v: must be a non-negative integer", value)
	}
	switch rev := rawRevision.(type) {
	case float64:
		if rev < 0 || rev != float64(int64(rev)) {
			return invalid(rev)
		}
		return int(rev), nil
	case float32:
		if rev < 0 || rev != float32(int64(rev)) {
			return invalid(rev)
		}
		return int(rev), nil
	case int:
		if rev < 0 { return invalid(rev) }
		return rev, nil
	case int32:
		if rev < 0 { return invalid(rev) }
		return int(rev), nil
	case int64:
		if rev < 0 { return invalid(rev) }
		return int(rev), nil
	case json.Number:
		parsed, err := strconv.Atoi(string(rev))
		if err != nil || parsed < 0 { return invalid(rev) }
		return parsed, nil
	case string:
		parsed, err := strconv.Atoi(rev)
		if err != nil || parsed < 0 { return invalid(rev) }
		return parsed, nil
	default:
		return 0, fmt.Errorf("invalid revision type %T: must be a non-negative integer", rawRevision)
	}
}
