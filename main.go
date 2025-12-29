// Package main is the entry point for the Kubernetes MCP server.
// Manage Kubernetes Cluster workloads via MCP.
// It initializes the MCP server, sets up the Kubernetes client,
// and registers the necessary handlers for various Kubernetes operations.
// It also starts the server to listen for incoming requests via stdio, SSE, or streamable-http transport.
// It uses the MCP Go library to create the server and handle requests.
// The server is capable of handling various Kubernetes operations
// such as listing resources, getting resource details, and retrieving logs.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/reza-gholizade/k8s-mcp-server/handlers"
	"github.com/reza-gholizade/k8s-mcp-server/pkg/helm"
	"github.com/reza-gholizade/k8s-mcp-server/pkg/k8s"
	"github.com/reza-gholizade/k8s-mcp-server/tools"

	"github.com/mark3labs/mcp-go/server"
)

// main initializes the Kubernetes client, sets up the MCP server with
// Kubernetes tool handlers, and starts the server in the configured mode.
func main() {
	// Parse command line flags
	var mode string
	var port string
	var readOnly bool
	var noK8s bool
	var noHelm bool

	flag.StringVar(&port, "port", getEnvOrDefault("SERVER_PORT", "8080"), "Server port")
	flag.StringVar(&mode, "mode", getEnvOrDefault("SERVER_MODE", "sse"), "Server mode: 'stdio', 'sse', or 'streamable-http'")
	flag.BoolVar(&readOnly, "read-only", false, "Enable read-only mode (disables write operations)")
	flag.BoolVar(&noK8s, "no-k8s", false, "Disable Kubernetes tools")
	flag.BoolVar(&noHelm, "no-helm", false, "Disable Helm tools")
	flag.Parse()

	// Validate flag combinations
	if noK8s && noHelm {
		fmt.Println("Error: Cannot disable both Kubernetes and Helm tools. At least one tool category must be enabled.")
		os.Exit(1)
	}

	// Log read-only mode status
	if readOnly {
		fmt.Println("Starting server in read-only mode - write operations disabled")
	}

	// Log disabled tool categories
	if noK8s {
		fmt.Println("Kubernetes tools disabled")
	}
	if noHelm {
		fmt.Println("Helm tools disabled")
	}

	// Create MCP server
	s := server.NewMCPServer(
		"MCP K8S & Helm Server",
		"1.0.0",
		server.WithResourceCapabilities(true, true), // Enable resource listing and subscription capabilities
	)

	// Create a Kubernetes client
	client, err := k8s.NewClient("")
	if err != nil {
		fmt.Printf("Failed to create Kubernetes client: %v\n", err)
		return
	}

	// Create Helm client with default kubeconfig path
	helmClient, err := helm.NewClient("")
	if err != nil {
		fmt.Printf("Failed to create Helm client: %v\n", err)
		return
	}

	// Register Kubernetes tools
	if !noK8s {
		s.AddTool(tools.GetAPIResourcesTool(), handlers.GetAPIResources(client))
		s.AddTool(tools.ListResourcesTool(), handlers.ListResources(client))
		s.AddTool(tools.GetResourcesTool(), handlers.GetResources(client))
		s.AddTool(tools.DescribeResourcesTool(), handlers.DescribeResources(client))
		s.AddTool(tools.GetPodsLogsTools(), handlers.GetPodsLogs(client))
		s.AddTool(tools.GetNodeMetricsTools(), handlers.GetNodeMetrics(client))
		s.AddTool(tools.GetPodMetricsTool(), handlers.GetPodMetrics(client))
		s.AddTool(tools.GetEventsTool(), handlers.GetEvents(client))
		s.AddTool(tools.GetIngressesTool(), handlers.GetIngresses(client))

		// Register write operations only if not in read-only mode
		if !readOnly {
			s.AddTool(tools.CreateOrUpdateResourceJSONTool(), handlers.CreateOrUpdateResourceJSON(client))
			s.AddTool(tools.CreateOrUpdateResourceYAMLTool(), handlers.CreateOrUpdateResourceYAML(client))
			s.AddTool(tools.DeleteResourceTool(), handlers.DeleteResource(client))
			s.AddTool(tools.RolloutRestartTool(), handlers.RolloutRestart(client))
		}
	}

	// Register Helm tools
	if !noHelm {
		s.AddTool(tools.HelmListTool(), handlers.HelmList(helmClient))
		s.AddTool(tools.HelmGetTool(), handlers.HelmGet(helmClient))
		s.AddTool(tools.HelmHistoryTool(), handlers.HelmHistory(helmClient))
		s.AddTool(tools.HelmRepoListTool(), handlers.HelmRepoList(helmClient))

		// Register write operations only if not in read-only mode
		if !readOnly {
			s.AddTool(tools.HelmInstallTool(), handlers.HelmInstall(helmClient))
			s.AddTool(tools.HelmUpgradeTool(), handlers.HelmUpgrade(helmClient))
			s.AddTool(tools.HelmUninstallTool(), handlers.HelmUninstall(helmClient))
			s.AddTool(tools.HelmRollbackTool(), handlers.HelmRollback(helmClient))
			s.AddTool(tools.HelmRepoAddTool(), handlers.HelmRepoAdd(helmClient))
		}
	}

	// Start server based on mode
	switch mode {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Printf("Failed to start stdio server: %v\n", err)
			return
		}
	case "sse":
		fmt.Printf("Starting server in SSE mode on port %s...\n", port)
		sse := server.NewSSEServer(s)
		if err := sse.Start(":" + port); err != nil {
			fmt.Printf("Failed to start SSE server: %v\n", err)
			return
		}
		fmt.Printf("SSE server started on port %s\n", port)
	case "streamable-http":
		fmt.Printf("Starting server in streamable-http mode on port %s...\n", port)
		streamableHTTP := server.NewStreamableHTTPServer(s, server.WithStateLess(true))
		if err := streamableHTTP.Start(":" + port); err != nil {
			fmt.Printf("Failed to start streamable-http server: %v\n", err)
			return
		}
		fmt.Printf("Streamable-http server started on port %s (endpoint: http://localhost:%s/mcp)\n", port, port)
	default:
		fmt.Printf("Unknown server mode: %s. Use 'stdio', 'sse', or 'streamable-http'.\n", mode)
		return
	}
}

// getEnvOrDefault returns the value of the environment variable or the default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
