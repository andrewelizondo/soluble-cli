//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"

	"github.com/soluble-ai/soluble-cli/cmd/root"
	"github.com/soluble-ai/soluble-cli/cmd/test"
	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	tool := test.NewTool(t, "tf-scan", "-d", "testdata", "--config-file", "/dev/null")
	tool.Must(tool.Run())
	lines := strings.Split(tool.Out.String(), "\n")
	assert := assert.New(t)
	assert.Greater(len(lines), 1)
	assert.Contains(lines[0], "SID ")
}

func TestScanUploadJSON(t *testing.T) {
	test.RequireAPIToken(t)
	tool := test.NewTool(t, "tf-scan", "-d", "testdata", "--config-file", "/dev/null", "--format", "json").
		WithUpload(true)
	tool.Must(tool.Run())
	n := tool.JSON()
	assert := assert.New(t)
	if !assert.Equal(1, n.Size()) {
		return
	}
	assmt := n.Get(0)
	assert.NotEmpty(assmt.Path("appUrl").AsText())
	assert.NotEmpty(assmt.Path("assessmentId").AsText())
	assert.Greater(assmt.Path("findings").Size(), 1)
}

func TestCheckovVarFile(t *testing.T) {
	tool := test.NewTool(t, "tf-scan", "-d", "testdata/withvars", "--var-file", "testdata/withvars/pass.tfvars")
	tool.ExtraArgs = []string{"--check", "CKV_AWS_20"}
	tool.Must(tool.Run())
	lines := strings.Split(tool.Out.String(), "\n")
	if assert.Len(t, lines, 3) {
		assert.Contains(t, lines[1], "PASS")
	}
}

func TestFail(t *testing.T) {
	assert := assert.New(t)
	test.RequireAPIToken(t)
	var exitCode int
	root.ExitFunc = func(code int) { exitCode = code }
	defer func() { root.ExitFunc = os.Exit }()
	tool := test.NewTool(t, "tf-scan", "-d", "testdata", "--config-file", "/dev/null", "--fail", "high").
		WithUpload(true)
	tool.Must(tool.Run())
	assert.Equal(2, exitCode)
}

func TestTfsec(t *testing.T) {
	assert := assert.New(t)
	tool := test.NewTool(t, "tf-scan", "tfsec", "-d", "testdata", "--config-file", "/dev/null",
		"--format", "json")
	tool.Must(tool.Run())
	n := tool.JSON()
	assert.Greater(n.Get(0).Path("findings").Size(), 1)
}
