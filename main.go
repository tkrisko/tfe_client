package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tfe "github.com/hashicorp/go-tfe"
)

type Command string

const (
	Workspace      Command = "workspace"
	OAuthClient    Command = "oauth_clients"
	List           Command = "list"
	Create         Command = "create"
	Delete         Command = "delete"
	AddRepo        Command = "add_repo"
	Get            Command = "get"
	AddTFEVariable Command = "add_tfe_var"
	AddEnvVariable Command = "add_env_var"
)

type Connection struct {
	Client *tfe.Client
	Org    string
}

func NewConnection(config *tfe.Config, org string) (*Connection, error) {
	client, err := tfe.NewClient(config)
	return &Connection{
		Client: client,
		Org:    org,
	}, err
}

func (c *Connection) ListWorkspaces() []string {
	ctx := context.Background()
	var ws *tfe.WorkspaceList
	var err error
	options := &tfe.WorkspaceListOptions{
		ListOptions: tfe.ListOptions{PageNumber: 1},
	}
	var workspaces []string
	for {
		ws, err = c.Client.Workspaces.List(ctx, c.Org, options)
		if err != nil {
			log.Fatal(err)
		}
		for _, w := range ws.Items {
			workspaces = append(workspaces, w.Name)
		}
		options.ListOptions.PageNumber = ws.NextPage
		if ws.NextPage == 0 {
			break
		}
	}
	return workspaces
}

func (c *Connection) CreateWorkspace(name string, workingDir string) {
	ctx := context.Background()
	// Create a new workspace
	w, err := c.Client.Workspaces.Create(ctx, c.Org, tfe.WorkspaceCreateOptions{
		Name:             tfe.String(name),
		AutoApply:        tfe.Bool(false),
		WorkingDirectory: &workingDir,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", w)
}

func (c *Connection) ReadWorkspace(name string) (*tfe.Workspace, error) {
	ctx := context.Background()
	return c.Client.Workspaces.Read(ctx, c.Org, name)
}

func (c *Connection) GetWorkspace(name string) string {
	ws, err := c.ReadWorkspace(name)
	if err != nil {
		log.Fatal(err)
	}
	var branch, repo_identifier string
	if ws.VCSRepo == nil {
		branch = "undef"
		repo_identifier = "undef"
	}
	return fmt.Sprintf("%s %s %s %s", ws.Name, ws.WorkingDirectory, branch, repo_identifier)
}

func (c *Connection) UpdateWorkspace(name string, options *tfe.WorkspaceUpdateOptions) error {
	w, err := c.ReadWorkspace(name)
	ctx := context.Background()
	if err != nil {
		return err
	}
	_, err = c.Client.Workspaces.Update(ctx, c.Org, w.Name, *options)
	return err
}

func (c *Connection) ListOAuthClients() []string {
	ctx := context.Background()
	options := &tfe.OAuthClientListOptions{
		ListOptions: tfe.ListOptions{PageNumber: 1},
	}
	var err error
	var ts *tfe.OAuthClientList
	var oAuthClients []string
	for {
		ts, err = c.Client.OAuthClients.List(ctx, c.Org, options)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range ts.Items {
			oAuthClients = append(oAuthClients, fmt.Sprintf("%s %s", *t.Name, t.ID))
		}
		options.ListOptions.PageNumber = ts.NextPage
		if ts.NextPage == 0 {
			break
		}
	}
	return oAuthClients
}

func (c *Connection) ReadOAuthClient(name string) (*tfe.OAuthClient, error) {
	ctx := context.Background()
	return c.Client.OAuthClients.Read(ctx, name)
}

func (c *Connection) GetVCSProviderFromOAuthClient(clientName string, branch string, repoIdentifier string) (*tfe.VCSRepoOptions, error) {
	oauthclient, err := c.ReadOAuthClient(clientName)
	if err != nil {
		return nil, err
	}
	vcsrepo := &tfe.VCSRepoOptions{
		Branch:       &branch,
		Identifier:   &repoIdentifier,
		OAuthTokenID: &oauthclient.OAuthTokens[0].ID,
	}
	return vcsrepo, nil
}

func (c *Connection) addTerraformVariable(name *string, wsName *string, value *string, description *string, hcl *bool, sensitive *bool, category *tfe.CategoryType) (*tfe.Variable, error) {
	ctx := context.Background()
	options := &tfe.VariableCreateOptions{
		Key:         name,
		Description: description,
		HCL:         hcl,
		Category:    category,
		Value:       value,
		Sensitive:   sensitive,
	}
	w, err := c.ReadWorkspace(*wsName)
	v, err := c.Client.Variables.Create(ctx, w.ID, *options)
	return v, err
}

func (c *Connection) AddTerraformVariable(name string, wsName string, value string, description string, hcl bool, sensitive bool) error {
	_, err := c.addTerraformVariable(&name, &wsName, &value, &description, &hcl, &sensitive, tfe.Category("terraform"))
	return err
}

func (c *Connection) AddEnvironmentVariable(name string, wsName string, value string, description string, sensitive bool) error {
	hcl := false
	_, err := c.addTerraformVariable(&name, &wsName, &value, &description, &hcl, &sensitive, tfe.Category("env"))
	return err
}

func main() {
	var tfeURL, tfeToken, tfeOrg string
	var command, subCommand string
	var help = flag.Bool("help", false, "Show help")
	var workspaceName, workingDir, oAuthId, branch, repoURL string
	var varDescription, varName, varValue string
	var isHCL, isSensitive bool

	flag.StringVar(&tfeURL, "tfe_url", os.Getenv("TFE_URL"), "Terraform organization. TFE_URL environment variable or given flag")
	flag.StringVar(&tfeToken, "tfe_token", os.Getenv("TFE_TOKEN"), "Terraform token. Terraform token. TFE_TOKEN environment variable or given flag")
	flag.StringVar(&tfeOrg, "tfe_org", os.Getenv("TFE_ORG"), "Terraform Organisation. TFE_ORG environment variable or given flag")
	flag.StringVar(&workspaceName, "workspace_name", "", "Workspace Name")
	flag.StringVar(&workingDir, "work_dir", "", "Working directory")
	flag.StringVar(&oAuthId, "oauth_client_id", "", "Working directory")
	flag.StringVar(&branch, "branch", "", "Repository branch")
	flag.StringVar(&repoURL, "repo_url", "", "Repository URL in format organization/repository")
	flag.BoolVar(&isHCL, "is_hcl", false, "Make variable hcl")
	flag.BoolVar(&isSensitive, "is_sensitive", false, "Make variable hcl")
	flag.StringVar(&varName, "var_name", "", "Variable name")
	flag.StringVar(&varDescription, "var_description", "", "Variable description")
	flag.StringVar(&varValue, "var_value", "", "Variable value")

	flag.Usage = func() {
		message := fmt.Sprintf("Usage of %s:\n", os.Args[0])
		message = message + (fmt.Sprintf("  workspace [create|list|get]\n"))
		message = message + (fmt.Sprintf("    -workspace_name\n\t%s\n", flag.CommandLine.Lookup("workspace_name").Usage))
		message = message + (fmt.Sprintf("    -work_dir\n\t%s\n", flag.CommandLine.Lookup("work_dir").Usage))
		message = message + (fmt.Sprintf("  add_repo\n"))
		message = message + (fmt.Sprintf("    -workspace_name.\n\t%s\n", flag.CommandLine.Lookup("workspace_name").Usage))
		message = message + (fmt.Sprintf("    -branch\n\t%s\n", flag.CommandLine.Lookup("branch").Usage))
		message = message + (fmt.Sprintf("    -repo_url\n\t%s\n", flag.CommandLine.Lookup("repo_url").Usage))
		message = message + (fmt.Sprintf("  add_tfe_var\n"))
		message = message + (fmt.Sprintf("    -workspace_name\n\t%s\n", flag.CommandLine.Lookup("workspace_name").Usage))
		message = message + (fmt.Sprintf("    -var_name\n\t%s\n", flag.CommandLine.Lookup("var_name").Usage))
		message = message + (fmt.Sprintf("    -var_value\n\t%s\n", flag.CommandLine.Lookup("var_value").Usage))
		message = message + (fmt.Sprintf("    -var_description\n\t%s\n", flag.CommandLine.Lookup("var_description").Usage))
		message = message + (fmt.Sprintf("    -is_hcl.\n\t%s\n", flag.CommandLine.Lookup("is_hcl").Usage))
		message = message + (fmt.Sprintf("    -is_sensitive\n\t%s\n", flag.CommandLine.Lookup("is_sensitive").Usage))
		message = message + (fmt.Sprintf("  add_env_var\n"))
		message = message + (fmt.Sprintf("    -workspace_name\n\t%s\n", flag.CommandLine.Lookup("workspace_name").Usage))
		message = message + (fmt.Sprintf("    -var_name\n\t%s\n", flag.CommandLine.Lookup("var_name").Usage))
		message = message + (fmt.Sprintf("    -var_value\n\t%s\n", flag.CommandLine.Lookup("var_value").Usage))
		message = message + (fmt.Sprintf("    -var_description\n\t%s\n", flag.CommandLine.Lookup("var_description").Usage))
		message = message + (fmt.Sprintf("    -is_sensitive\n\t%s\n", flag.CommandLine.Lookup("is_sensitive").Usage))
		message = message + (fmt.Sprintf("  -tfe_url\n\t%s\n", flag.CommandLine.Lookup("tfe_url").Usage))
		message = message + (fmt.Sprintf("  -tfe_org\n\t%s\n", flag.CommandLine.Lookup("tfe_org").Usage))
		message = message + (fmt.Sprintf("  -tfe_token\n\t%s\n", flag.CommandLine.Lookup("tfe_token").Usage))

		fmt.Println(message)
	}
	//remove commands before parsing flags
	if len(os.Args) > 2 {
		command = os.Args[1]
		subCommand = os.Args[2]
		if strings.Index(command, "-") == -1 && strings.Index(subCommand, "-") == -1 {
			os.Args = os.Args[2:]
		}
	}

	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if len(tfeToken) == 0 || len(tfeURL) == 0 {
		fmt.Println("TFE_TOKEN or TFE_URL are missing")
		flag.Usage()
		os.Exit(0)
	}

	config := &tfe.Config{
		Address:           tfeURL,
		Token:             tfeToken,
		RetryServerErrors: true,
	}

	client, err := NewConnection(config, tfeOrg)
	if err != nil {
		log.Fatal(err)
	}

	switch command {
	case string(Workspace):
		switch subCommand {
		case string(List):
			for _, w := range client.ListWorkspaces() {
				fmt.Println(w)
			}
		case string(Create):
			client.CreateWorkspace(workspaceName, workingDir)
		case string(Get):
			fmt.Println(client.GetWorkspace(workspaceName))
		case string(AddRepo):
			vcsRepo, _ := client.GetVCSProviderFromOAuthClient(oAuthId, branch, repoURL)
			options := &tfe.WorkspaceUpdateOptions{
				VCSRepo: vcsRepo,
			}
			err = client.UpdateWorkspace(workspaceName, options)
			if err != nil {
				log.Fatal(err)
			}
		case string(AddTFEVariable):
			err = client.AddTerraformVariable(varName, workspaceName, varValue, varDescription, isHCL, isSensitive)
			if err != nil {
				log.Fatal(err)
			}
		case string(AddEnvVariable):
			err = client.AddEnvironmentVariable(varName, workspaceName, varValue, varDescription, isSensitive)
			if err != nil {
				log.Fatal(err)
			}
		}

	case string(OAuthClient):
		switch subCommand {
		case string(List):
			for _, w := range client.ListOAuthClients() {
				fmt.Println(w)
			}
		}

	}
}
