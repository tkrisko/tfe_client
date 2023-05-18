package main

import (
	"context"
	"encoding/json"
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
	Run            Command = "run"
	OAuthClient    Command = "oauth_clients"
	List           Command = "list"
	Create         Command = "create"
	Delete         Command = "delete"
	AddRepo        Command = "add_repo"
	Get            Command = "get"
	AddTFEVariable Command = "add_tfe_var"
	AddEnvVariable Command = "add_env_var"
	Plan           Command = "plan"
	Discard        Command = "discard"
	Apply          Command = "apply"
	Cancel         Command = "cancel"
	ListRuns       Command = "list_runs"
	ApplyStatus    Command = "apply_status"
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

func (c *Connection) GetWorkspace(name string) []byte {
	ws, err := c.ReadWorkspace(name)
	if err != nil {
		log.Fatal(err)
	}
	var wsMap map[string]interface{}
	var js []byte
	var branch, repo_identifier string
	wsMap = make(map[string]interface{})
	if ws.VCSRepo != nil {
		branch = ws.VCSRepo.Branch
		repo_identifier = ws.VCSRepo.Identifier
	}
	wsMap["Name"] = ws.Name
	wsMap["WorkingDirectory"] = ws.WorkingDirectory
	wsMap["Branch"] = branch
	wsMap["RepoID"] = repo_identifier
	wsMap["Locked"] = ws.Locked
	js, err = json.Marshal(wsMap)
	if err != nil {
		log.Fatal(err)
	}
	return js
}

func (c *Connection) UpdateWorkspace(name string, options *tfe.WorkspaceUpdateOptions) error {
	w, err := c.ReadWorkspace(name)
	ctx := context.Background()
	if err != nil {
		return err
	}
	w, err = c.Client.Workspaces.Update(ctx, c.Org, w.Name, *options)
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

func (c *Connection) RunPlan(name string, message string) (string, error) {
	w, err := c.ReadWorkspace(name)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	options := tfe.RunCreateOptions{
		Workspace: w,
		Message:   &message,
	}
	r, err := c.Client.Runs.Create(ctx, options)
	return r.ID, err
}

func (c *Connection) DiscardRun(runID string, message string) error {
	ctx := context.Background()
	options := tfe.RunDiscardOptions{
		Comment: &message,
	}

	err := c.Client.Runs.Discard(ctx, runID, options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) CancelRun(runID string, message string) error {
	ctx := context.Background()
	options := tfe.RunCancelOptions{
		Comment: &message,
	}

	err := c.Client.Runs.Cancel(ctx, runID, options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) ListRuns(workspaceName string) []byte {
	ctx := context.Background()
	w, err := c.ReadWorkspace(workspaceName)
	if err != nil {
		log.Fatal(err)
	}

	options := &tfe.RunListOptions{
		ListOptions: tfe.ListOptions{PageNumber: 1},
	}

	var rs *tfe.RunList
	var runs []map[string]string
	var run map[string]string
	var js []byte
	for {
		rs, err = c.Client.Runs.List(ctx, w.ID, options)
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range rs.Items {
			run = make(map[string]string)
			run["ID"] = r.ID
			run["Status"] = string(r.Status)
			run["CreatedAt"] = fmt.Sprintf("%s", r.CreatedAt)
			runs = append(runs, run)

		}
		options.ListOptions.PageNumber = rs.NextPage
		if rs.NextPage == 0 {
			break
		}
	}
	js, err = json.Marshal(&runs)
	if err != nil {
		log.Fatal(err)
	}
	return js
}

func (c *Connection) GetPlan(runID string) string {
	ctx := context.Background()
	r, err := c.Client.Runs.Read(ctx, runID)
	if err != nil {
		log.Fatal(err)
	}
	p, err := c.Client.Plans.ReadJSONOutput(ctx, r.Plan.ID)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%s", p)
}

func (c *Connection) ApplyRun(runID string, message string) error {
	ctx := context.Background()
	options := tfe.RunApplyOptions{
		Comment: &message,
	}

	err := c.Client.Runs.Apply(ctx, runID, options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) GetApply(runID string) string {
	ctx := context.Background()
	r, err := c.Client.Runs.Read(ctx, runID)
	if err != nil {
		log.Fatal(err)
	}
	a, err := c.Client.Applies.Read(ctx, r.Apply.ID)
	if err != nil {
		log.Fatal(err)
	}
	js, err := json.Marshal(a)
	return fmt.Sprintf("%s", js)
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
	var varDescription, varName, varValue, msg, planID string
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
	flag.StringVar(&msg, "message", "", "Plan messages")
	flag.StringVar(&planID, "plan_id", "", "Plan id")

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
			fmt.Printf("%s", client.GetWorkspace(workspaceName))
		case string(AddRepo):
			vcsRepo, err := client.GetVCSProviderFromOAuthClient(oAuthId, branch, repoURL)
			if err != nil {
				log.Fatal(err)
			}
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
		case string(Plan):
			id, err := client.RunPlan(workspaceName, msg)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Plan id %s started\n", id)
		default:
			flag.Usage()
		}

	case string(OAuthClient):
		switch subCommand {
		case string(List):
			for _, w := range client.ListOAuthClients() {
				fmt.Println(w)
			}
		}
	case string(Run):
		switch subCommand {
		case string(Discard):
			err := client.DiscardRun(planID, msg)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Run id %s discarded\n", planID)
		case string(Cancel):
			err := client.CancelRun(planID, msg)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Run id %s cancelled\n", planID)
		case string(Get):
			fmt.Println(client.GetPlan(planID))
		case string(ApplyStatus):
			fmt.Println(client.GetApply(planID))
		case string(Apply):
			err := client.ApplyRun(planID, msg)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Run id %s applied\n", planID)
		case string(List):
			fmt.Printf("%s", client.ListRuns(workspaceName))
		default:
			flag.Usage()
		}
	default:
		flag.Usage()
	}
}
