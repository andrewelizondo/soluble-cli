// Copyright 2021 Soluble Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inventory

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTerraform(t *testing.T) {
	assert := assert.New(t)
	m := &Manifest{}
	m.scan(filepath.Join("testdata", "tf"), &terraformDetector{})
	assert.ElementsMatch(m.TerraformRootModules.Values(), []string{
		"r1", "r1j", "r2",
	})
	assert.ElementsMatch(m.TerraformModules.Values(), []string{
		"r1", "r1j", "r2", "m1",
	})
}

func TestTerraformIgnroed(t *testing.T) {
	assert := assert.New(t)
	td := &terraformDetector{}
	assert.True(td.isIgnoredDirectory("foo/.terraform/main.tf"))
	assert.False(td.isIgnoredDirectory("main.tf"))
	assert.True(td.isIgnoredDirectory(".external_modules/foo/main.tf"))
}
