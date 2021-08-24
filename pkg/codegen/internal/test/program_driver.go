package test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/utils"
)

type programTest struct {
	ProgramFile    string
	Description    string
	Skip           codegen.StringSet
	ExpectNYIDiags codegen.StringSet
}

var testdataPath = filepath.Join("..", "internal", "test", "testdata")

var programTests = []programTest{
	{
		ProgramFile:    "aws-s3-folder",
		Description:    "AWS S3 Folder",
		ExpectNYIDiags: codegen.NewStringSet("python", "nodejs", "dotnet"),
	},
	{
		ProgramFile: "aws-eks",
		Description: "AWS EKS",
	},
	{
		ProgramFile: "aws-fargate",
		Description: "AWS Fargate",
	},
	{
		ProgramFile: "aws-s3-logging",
		Description: "AWS S3 with logging",
	},
	{
		ProgramFile: "aws-webserver",
		Description: "AWS Webserver",
	},
	{
		ProgramFile: "azure-native",
		Description: "Azure Native",
		Skip:        codegen.NewStringSet("go"),
	},
	{
		ProgramFile: "azure-sa",
		Description: "Azure SA",
	},
	{
		ProgramFile: "kubernetes-operator",
		Description: "K8s Operator",
	},
	{
		ProgramFile: "kubernetes-pod",
		Description: "K8s Pod",
	},
	{
		ProgramFile: "kubernetes-template",
		Description: "K8s Template",
	},
	{
		ProgramFile: "random-pet",
		Description: "Random Pet",
	},
	{
		ProgramFile: "resource-options",
		Description: "Resource Options",
	},
	{
		ProgramFile: "secret",
		Description: "Secret",
	},
	{
		ProgramFile: "functions",
		Description: "Functions",
	},
}

type langConfig struct {
	extension  string
	outputFile string
}

// TestProgramCodegen runs the complete set of program code generation tests against a particular
// language's code generator.
//
// A program code generation test consists of a PCL file (.pp extension) and a set of expected outputs
// for each language.
//
// The PCL file is the only piece that must be manually authored. Once the schema has been written, the expected outputs
// can be generated by running `PULUMI_ACCEPT=true go test ./..." from the `pkg/codegen` directory.
func TestProgramCodegen(
	t *testing.T,
	language string,
	genProgram func(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error),
) {
	for _, tt := range programTests {
		t.Run(tt.Description, func(t *testing.T) {
			if tt.Skip.Has(language) {
				t.Skip()
				return
			}

			expectNYIDiags := false
			if tt.ExpectNYIDiags.Has(language) {
				expectNYIDiags = true
			}

			var cfg langConfig

			switch language {
			case "python":
				cfg = langConfig{
					extension:  "py",
					outputFile: "__main__.py",
				}
			case "nodejs":
				cfg = langConfig{
					extension:  "ts",
					outputFile: "index.ts",
				}
			case "go":
				cfg = langConfig{
					extension:  "go",
					outputFile: "main.go",
				}
			case "dotnet":
				cfg = langConfig{
					extension:  "cs",
					outputFile: "MyStack.cs",
				}
			default:
				t.Fatalf("language %s not recognized", language)
			}

			pclFile := filepath.Join(testdataPath, tt.ProgramFile+".pp")
			contents, err := ioutil.ReadFile(pclFile)
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}

			expectedFile := pclFile + "." + cfg.extension
			expected, err := ioutil.ReadFile(expectedFile)
			if err != nil && os.Getenv("PULUMI_ACCEPT") == "" {
				t.Fatalf("could not read %v: %v", expectedFile, err)
			}

			parser := syntax.NewParser()
			err = parser.ParseFile(bytes.NewReader(contents), tt.ProgramFile+".pp")
			if err != nil {
				t.Fatalf("could not read %v: %v", pclFile, err)
			}
			if parser.Diagnostics.HasErrors() {
				t.Fatalf("failed to parse files: %v", parser.Diagnostics)
			}

			program, diags, err := hcl2.BindProgram(parser.Files, hcl2.PluginHost(utils.NewHost(testdataPath)))
			if err != nil {
				t.Fatalf("could not bind program: %v", err)
			}
			if diags.HasErrors() {
				t.Fatalf("failed to bind program: %v", diags)
			}

			files, diags, err := genProgram(program)
			assert.NoError(t, err)
			if expectNYIDiags {
				var tmpDiags hcl.Diagnostics
				for _, d := range diags {
					if !strings.HasPrefix(d.Summary, "not yet implemented") {
						tmpDiags = append(tmpDiags, d)
					}
				}
				diags = tmpDiags
			}
			if diags.HasErrors() {
				t.Fatalf("failed to generate program: %v", diags)
			}

			if os.Getenv("PULUMI_ACCEPT") != "" {
				err := ioutil.WriteFile(expectedFile, files[cfg.outputFile], 0600)
				require.NoError(t, err)
				return
			}

			assert.Equal(t, string(expected), string(files[cfg.outputFile]))
		})
	}
}