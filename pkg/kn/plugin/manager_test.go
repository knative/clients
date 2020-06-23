// Copyright © 2018 The Knative Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/assert"
)

var testPluginScriptUnix = `#!/bin/bash
echo "OK $*"
`
var testPluginScriptWindows = `
print "OK"
`

type testContext struct {
	pluginsDir    string
	pluginManager *Manager
}

type testPlugin struct {
	parts []string
}

func (t testPlugin) Name() string { return strings.Join(t.parts, " ") }
func (t testPlugin) Execute(args []string) error { return nil}
func (t testPlugin) Description() (string, error) { return "desc: " + t.Name(), nil }
func (t testPlugin) CommandParts() []string { return t.parts }
func (t testPlugin) Path() string { return "" }

var _ Plugin = testPlugin{}

func TestEmptyFind(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)

	plugin, err := ctx.pluginManager.FindPlugin([]string{})
	assert.NilError(t, err)
	assert.Equal(t, plugin, nil)
	assert.Assert(t, ctx.pluginManager.PluginsDir() != "")
	assert.Assert(t, !ctx.pluginManager.LookupInPath())
}

func TestLookupInPluginsDir(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)
	createTestPlugin(t, "kn-test", ctx)

	plugin, err := ctx.pluginManager.FindPlugin([]string{"test"})
	assert.NilError(t, err)
	assert.Assert(t, plugin != nil)
	assert.Assert(t, plugin.CommandParts()[0] == "test")

	out, err := executePlugin(plugin, []string{})
	assert.NilError(t, err)
	assert.Equal(t, out, "OK \n")
}

func TestFindWithNotFoundResult(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)

	plugin, err := ctx.pluginManager.FindPlugin([]string{"bogus", "plugin", "name"})
	assert.Assert(t, plugin == nil, "no plugin should be found")
	assert.NilError(t, err, "no error expected")
}

func TestFindPluginInPath(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)

	// Prepare PATH
	tmpPathDir, cleanupFunc := preparePathDirectory(t)
	defer cleanupFunc()

	createTestPluginInDirectory(t, "kn-path-test", tmpPathDir)
	pluginCommands := []string{"path", "test"}

	// Enable lookup --> find plugin
	ctx.pluginManager.lookupInPath = true
	plugin, err := ctx.pluginManager.FindPlugin(pluginCommands)
	assert.NilError(t, err)
	assert.Assert(t, plugin != nil)
	desc, err := plugin.Description()
	assert.NilError(t, err)
	assert.Assert(t, desc != "")
	assert.Equal(t, plugin.Path(), filepath.Join(tmpPathDir, "kn-path-test"))
	assert.DeepEqual(t, plugin.CommandParts(), pluginCommands)

	// Disable lookup --> no plugin
	ctx.pluginManager.lookupInPath = false
	plugin, err = ctx.pluginManager.FindPlugin(pluginCommands)
	assert.NilError(t, err)
	assert.Assert(t, plugin == nil)
}

func TestFindPluginInternally(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)

	// Initialize registered plugins
	plugins := PluginList{
		testPlugin{[]string{"a", "b"}},
		testPlugin{[]string{"a"}},
	}
	oldPlugins := InternalPlugins
	defer func() {
		InternalPlugins = oldPlugins
	}()
	InternalPlugins = plugins

	data := []struct {
		parts []string
		name string
	} {
		{ []string{ "a", "b"}, "a b" },
		{ []string{ "a"}, "a" },
		{ []string{ "a", "c"}, "a" },
	}
	for _, d := range data {
		plugin, err := ctx.pluginManager.FindPlugin(d.parts)
		assert.NilError(t, err)
		assert.Assert(t, plugin != nil)
		assert.Equal(t, plugin.Name(), d.name)
		assert.DeepEqual(t, plugin.CommandParts(), strings.Split(d.name, " "))
	}
}

func TestPluginExecute(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)
	createTestPlugin(t, "kn-test_with_dash-longer", ctx)

	plugin, err := ctx.pluginManager.FindPlugin([]string{"test-with-dash", "longer"})
	assert.NilError(t, err)
	out, err := executePlugin(plugin, []string{"arg1", "arg2"})
	assert.NilError(t, err)
	assert.Equal(t, out, "OK arg1 arg2\n")
}

func TestPluginListForCommandGroup(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)
	createTestPlugin(t, "kn-service-log_2", ctx)

	pluginList, err := ctx.pluginManager.ListPluginsForCommandGroup([]string{"service"})
	assert.NilError(t, err)
	assert.Assert(t, pluginList.Len() == 1)
	assert.Equal(t, pluginList[0].Name(), "kn-service-log_2")
	pluginList, err = ctx.pluginManager.ListPluginsForCommandGroup([]string{})
	assert.NilError(t, err)
	assert.Assert(t, pluginList.Len() == 0)
}

func TestPluginHelpMessage(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)
	createTestPlugin(t, "kn-service-log_2", ctx)
	createTestPlugin(t, "kn-admin", ctx)

	funcs := *ctx.pluginManager.HelpTemplateFuncs()
	f := funcs["listPlugins"]
	assert.Assert(t, f != nil)
	listPluginsFunc := ctx.pluginManager.listPluginsHelpMessage()

	root := &cobra.Command{
		Use: "kn",
	}
	serviceCmd := &cobra.Command{
		Use: "service",
	}
	serviceCreateCmd := &cobra.Command{
		Use: "create",
	}
	serviceCmd.AddCommand(serviceCreateCmd)
	root.AddCommand(serviceCmd)

	helpRoot := listPluginsFunc(root)
	re := regexp.MustCompile("^\\s*admin\\s.*admin")
	assert.Assert(t, re.MatchString(helpRoot))

	helpService := listPluginsFunc(serviceCmd)
	println(helpService)
	re = regexp.MustCompile("^\\s*log-2\\s.*kn-service-log_2")
	assert.Assert(t, re.MatchString(helpService))

	helpServiceCreate := listPluginsFunc(serviceCreateCmd)
	assert.Assert(t, len(helpServiceCreate) == 0)
}

func TestPluginList(t *testing.T) {
	ctx := setup(t)
	defer cleanup(t, ctx)

	// Plugin in plugin's dr
	createTestPlugin(t, "kn-zz-test_in_dir", ctx)

	// Plugin in Path
	tmpPathDir, cleanupFunc := preparePathDirectory(t)
	defer cleanupFunc()
	createTestPluginInDirectory(t, "kn-aa-path-test", tmpPathDir)

	// Enable lookup --> Both plugins found
	ctx.pluginManager.lookupInPath = true
	pluginList, err := ctx.pluginManager.ListPlugins()
	assert.NilError(t, err)
	assert.Assert(t, pluginList != nil)
	assert.Equal(t, len(pluginList), 2, "both plugins found (in dir + in path)")
	assert.Equal(t, pluginList[0].Name(), "kn-aa-path-test", "first plugin is alphabetically smallest (list is sorted)")
	assert.DeepEqual(t, pluginList[0].CommandParts(), []string{"aa", "path", "test"})
	assert.Equal(t, pluginList[1].Name(), "kn-zz-test_in_dir", "second plugin is alphabetically greater (list is sorted)")
	assert.DeepEqual(t, pluginList[1].CommandParts(), []string{"zz", "test-in-dir"})

	// Disable lookup --> Only one plugin found
	ctx.pluginManager.lookupInPath = false
	pluginList, err = ctx.pluginManager.ListPlugins()
	assert.NilError(t, err)
	assert.Assert(t, pluginList != nil)
	assert.Equal(t, len(pluginList), 1, "1 plugin found (in dir)")
	assert.Equal(t, pluginList[0].Name(), "kn-zz-test_in_dir", "second plugin is alphabetically greater (list is sorted)")
	assert.DeepEqual(t, pluginList[0].CommandParts(), []string{"zz", "test-in-dir"})
}

// ====================================================================
// Private

func setup(t *testing.T) testContext {
	return setupWithPathLookup(t, false)
}

func setupWithPathLookup(t *testing.T, lookupInPath bool) testContext {
	tmpPathDir, err := ioutil.TempDir("", "plugin_list")
	assert.NilError(t, err)
	return testContext{
		pluginsDir:    tmpPathDir,
		pluginManager: NewManager(tmpPathDir, lookupInPath),
	}
}

func cleanup(t *testing.T, ctx testContext) {
	err := os.RemoveAll(ctx.pluginsDir)
	assert.NilError(t, err)
}

func executePlugin(plugin Plugin, args []string) (string, error) {
	rescueStdout := os.Stdout
	defer (func() { os.Stdout = rescueStdout })()

	r, w, _ := os.Pipe()
	os.Stdout = w

	err := plugin.Execute(args)
	w.Close()
	if err != nil {
		return "", err
	}
	out, _ := ioutil.ReadAll(r)
	return string(out), nil
}

// Prepare a directory and set the path to this directory
func preparePathDirectory(t *testing.T) (string, func()) {
	tmpPathDir, err := ioutil.TempDir("", "plugin_path")
	assert.NilError(t, err)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s%c%s", tmpPathDir, os.PathListSeparator, "fast-forward-this-year-plz"))
	return tmpPathDir, func() {
		os.RemoveAll(tmpPathDir)
		os.Setenv("PATH", oldPath)
	}
}

// CreateTestPlugin with name, script, and fileMode and return the tmp random path
func createTestPlugin(t *testing.T, name string, ctx testContext) string {
	return createTestPluginInDirectory(t, name, ctx.pluginsDir)
}

// CreateTestPluginInPath with name, path, script, and fileMode and return the tmp random path
func createTestPluginInDirectory(t *testing.T, name string, dir string) string {
	var nameExt, script string
	if runtime.GOOS == "windows" {
		nameExt = name + ".bat"
		script = testPluginScriptWindows
	} else {
		nameExt = name
		script = testPluginScriptUnix
	}
	fullPath := filepath.Join(dir, nameExt)
	err := ioutil.WriteFile(fullPath, []byte(script), 0777)
	assert.NilError(t, err)
	// Some extra files to feed the tests
	err = ioutil.WriteFile(filepath.Join(dir, "non-plugin-prefix-"+nameExt), []byte{}, 0555)
	assert.NilError(t, err)
	_, err = ioutil.TempDir(dir, "bogus-dir")
	assert.NilError(t, err)

	return fullPath
}
