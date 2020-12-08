package all

import (
	"time"

	"github.com/soluble-ai/go-jnode"
	"github.com/soluble-ai/soluble-cli/pkg/inventory"
	"github.com/soluble-ai/soluble-cli/pkg/log"
	"github.com/soluble-ai/soluble-cli/pkg/print"
	"github.com/soluble-ai/soluble-cli/pkg/tools"
	cfnpythonlint "github.com/soluble-ai/soluble-cli/pkg/tools/cfn-python-lint"
	"github.com/soluble-ai/soluble-cli/pkg/tools/checkov"
	"github.com/soluble-ai/soluble-cli/pkg/tools/iacinventory"
	"github.com/soluble-ai/soluble-cli/pkg/tools/secrets"
	"github.com/soluble-ai/soluble-cli/pkg/util"
	"github.com/spf13/cobra"
)

type Tool struct {
	tools.DirectoryBasedToolOpts
	PrintToolResults bool
	Skip             []string
	ToolPaths        map[string]string
}

var _ tools.Interface = &Tool{}

type SubordinateTool struct {
	tools.Interface
	Skip   bool
	Result *tools.Result
	Err    error
}

func (*Tool) Name() string {
	return "all"
}

func (t *Tool) Register(cmd *cobra.Command) {
	t.Internal = true
	t.DirectoryBasedToolOpts.Register(cmd)
	flags := cmd.Flags()
	flags.BoolVar(&t.PrintToolResults, "print-tool-results", false, "Print individual results from tools")
	flags.StringSliceVar(&t.Skip, "skip", nil, "Don't run these `tools` (command-separated or repeated.)")
	flags.StringToStringVar(&t.ToolPaths, "tool-paths", nil, "Explicitly specify the path to each tool in the form `tool=path`.")
}

func (t *Tool) CommandTemplate() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "Find infrastructure-as-code and scan with recommended tools",
		Long: `Find infrastructure-as-code and scan with the following tools:

Cloudformation templates - cfn-python-lint
Terraform                - checkov
Kuberentes manifests     - checkov
Everything               - secrets			
`,
		Example: `# To run a tool locally w/o using docker explicitly specify the tool path
... all --tool-paths checkov=checkov,cfn-python-lint=cfn-lint`,
	}
}

func (t *Tool) Run() (*tools.Result, error) {
	m := inventory.Do(t.GetDirectory())
	subTools := []SubordinateTool{
		{
			Interface: &iacinventory.Local{
				DirectoryBasedToolOpts: t.getDirectoryOpts(),
			},
		},
		{
			Interface: &checkov.Tool{
				DirectoryBasedToolOpts: t.getDirectoryOpts(),
			},
			Skip: m.TerraformRootModuleDirectories.Len() == 0 && m.KubernetesManifestDirectories.Len() == 0,
		},
		{
			Interface: &cfnpythonlint.Tool{
				DirectoryBasedToolOpts: t.getDirectoryOpts(),
				Templates:              m.CloudformationFiles.Values(),
			},
			Skip: m.CloudformationFiles.Len() == 0,
		},
		{
			Interface: &secrets.Tool{
				DirectoryBasedToolOpts: t.getDirectoryOpts(),
			},
		},
	}
	result := &tools.Result{
		Data:      jnode.NewObjectNode(),
		PrintPath: []string{"data"},
		PrintColumns: []string{
			"name", "run_duration", "findings_count", "error",
			"assessment_url"},
	}
	resultData := result.Data.PutArray("data")
	count := 0
	for _, st := range subTools {
		n := resultData.AppendObject()
		n.Put("skipped", st.Skip)
		n.Put("name", st.Name())
		if st.Skip || util.StringSliceContains(t.Skip, st.Name()) {
			n.Put("run_duration", "skipped")
			continue
		}
		count++
		opts := st.GetToolOptions()
		opts.UploadEnabled = t.UploadEnabled
		opts.OmitContext = t.OmitContext
		opts.ToolPath = t.ToolPaths[st.Name()]
		start := time.Now()
		st.Result, st.Err = st.GetToolOptions().RunTool(st)
		rd := time.Since(start).Truncate(time.Millisecond)
		n.Put("run_duration", rd.String())
		if st.Result != nil {
			opts.Path = st.Result.PrintPath
			opts.Columns = st.Result.PrintColumns
			if t.PrintToolResults {
				opts.PrintResult(st.Result.Data)
			}
			if pr, err := st.GetToolOptions().GetPrinter(); err == nil {
				if tp, ok := pr.(*print.TablePrinter); ok {
					n.Put("findings_count", len(tp.GetRows(st.Result.Data)))
				}
			}
			n.Put("assessment_url", st.Result.AssessmentURL)
		}
		if st.Err != nil {
			n.Put("error", st.Err.Error())
		}
	}
	log.Infof("Finished running {primary:%d} tools", count)
	return result, nil
}

func (t *Tool) getDirectoryOpts() tools.DirectoryBasedToolOpts {
	return tools.DirectoryBasedToolOpts{
		Directory: t.GetDirectory(),
	}
}
