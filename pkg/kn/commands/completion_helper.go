// Copyright © 2021 The Knative Authors
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

package commands

import (
	"strings"

	"github.com/spf13/cobra"
)

type completionConfig struct {
	params     *KnParams
	command    *cobra.Command
	args       []string
	toComplete string
}

var (
	resourceToFuncMap = map[string]func(config *completionConfig) []string{
		"broker":   completeBroker,
		"domain":   completeDomain,
		"revision": completeRevision,
		"route":    completeRoute,
		"service":  completeService,
		"trigger":  completeTrigger,
	}
)

// ResourceNameCompletionFunc will return a function that will autocomplete the name of
// the resource based on the subcommand
func ResourceNameCompletionFunc(p *KnParams) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {

		var use string
		if cmd.Parent() != nil {
			use = cmd.Parent().Name()
		}
		config := completionConfig{
			p,
			cmd,
			args,
			toComplete,
		}
		return config.getCompletion(use), cobra.ShellCompDirectiveNoFileComp
	}
}

func (config *completionConfig) getCompletion(parent string) []string {
	completionFunc := resourceToFuncMap[parent]
	if completionFunc == nil {
		return []string{}
	}
	return completionFunc(config)
}

func getTargetFlagValue(cmd *cobra.Command) string {
	flag := cmd.Flag("target")
	if flag == nil {
		return ""
	}
	return flag.Value.String()
}

func completeGitOps(config *completionConfig) (suggestions []string) {
	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewGitopsServingClient(namespace, getTargetFlagValue(config.command))
	if err != nil {
		return
	}
	serviceList, err := client.ListServices(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range serviceList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}

func completeService(config *completionConfig) (suggestions []string) {
	if getTargetFlagValue(config.command) != "" {
		return completeGitOps(config)
	}

	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewServingClient(namespace)
	if err != nil {
		return
	}
	serviceList, err := client.ListServices(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range serviceList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}

func completeBroker(config *completionConfig) (suggestions []string) {
	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewEventingClient(namespace)
	if err != nil {
		return
	}
	brokerList, err := client.ListBrokers(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range brokerList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}

func completeRevision(config *completionConfig) (suggestions []string) {
	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewServingClient(namespace)
	if err != nil {
		return
	}
	revisionList, err := client.ListRevisions(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range revisionList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}

func completeRoute(config *completionConfig) (suggestions []string) {
	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewServingClient(namespace)
	if err != nil {
		return
	}
	routeList, err := client.ListRoutes(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range routeList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}

func completeDomain(config *completionConfig) (suggestions []string) {
	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewServingV1alpha1Client(namespace)
	if err != nil {
		return
	}
	domainMappingList, err := client.ListDomainMappings(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range domainMappingList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}

func completeTrigger(config *completionConfig) (suggestions []string) {
	suggestions = make([]string, 0)
	if len(config.args) != 0 {
		return
	}
	namespace, err := config.params.GetNamespace(config.command)
	if err != nil {
		return
	}
	client, err := config.params.NewEventingClient(namespace)
	if err != nil {
		return
	}
	triggerList, err := client.ListTriggers(config.command.Context())
	if err != nil {
		return
	}
	for _, sug := range triggerList.Items {
		if !strings.HasPrefix(sug.Name, config.toComplete) {
			continue
		}
		suggestions = append(suggestions, sug.Name)
	}
	return
}
