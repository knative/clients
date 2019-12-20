// Copyright © 2019 The Knative Authors
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

package service

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"knative.dev/client/pkg/kn/commands"
	servinglib "knative.dev/client/pkg/serving"
	"knative.dev/client/pkg/wait"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	client_testing "k8s.io/client-go/testing"
	"knative.dev/serving/pkg/apis/serving/v1alpha1"
)

func fakeServiceCreate(args []string, withExistingService bool, sync bool) (
	action client_testing.Action,
	service *v1alpha1.Service,
	output string,
	err error) {
	knParams := &commands.KnParams{}
	nrGetCalled := 0
	cmd, fakeServing, buf := commands.CreateTestKnCommand(NewServiceCommand(knParams), knParams)
	fakeServing.AddReactor("get", "services",
		func(a client_testing.Action) (bool, runtime.Object, error) {
			nrGetCalled++
			if withExistingService || (sync && nrGetCalled > 1) {
				return true, &v1alpha1.Service{}, nil
			}
			return true, nil, api_errors.NewNotFound(schema.GroupResource{}, "")
		})
	fakeServing.AddReactor("create", "services",
		func(a client_testing.Action) (bool, runtime.Object, error) {
			createAction, ok := a.(client_testing.CreateAction)
			action = createAction
			if !ok {
				return true, nil, fmt.Errorf("wrong kind of action %v", a)
			}
			service, ok = createAction.GetObject().(*v1alpha1.Service)
			if !ok {
				return true, nil, errors.New("was passed the wrong object")
			}
			return true, service, nil
		})
	fakeServing.AddReactor("update", "services",
		func(a client_testing.Action) (bool, runtime.Object, error) {
			updateAction, ok := a.(client_testing.UpdateAction)
			action = updateAction
			if !ok {
				return true, nil, fmt.Errorf("wrong kind of action %v", a)
			}
			service, ok = updateAction.GetObject().(*v1alpha1.Service)
			if !ok {
				return true, nil, errors.New("was passed the wrong object")
			}
			return true, service, nil
		})
	if sync {
		fakeServing.AddWatchReactor("services",
			func(a client_testing.Action) (bool, watch.Interface, error) {
				watchAction := a.(client_testing.WatchAction)
				_, found := watchAction.GetWatchRestrictions().Fields.RequiresExactMatch("metadata.name")
				if !found {
					return true, nil, errors.New("no field selector on metadata.name found")
				}
				w := wait.NewFakeWatch(getServiceEvents("test-service"))
				w.Start()
				return true, w, nil
			})
		fakeServing.AddReactor("get", "services",
			func(a client_testing.Action) (bool, runtime.Object, error) {
				return true, &v1alpha1.Service{}, nil
			})
	}
	cmd.SetArgs(args)
	err = cmd.Execute()
	if err != nil {
		output = err.Error()
		return
	}
	output = buf.String()
	return
}

func getServiceEvents(name string) []watch.Event {
	return []watch.Event{
		{watch.Added, wait.CreateTestServiceWithConditions(name, corev1.ConditionUnknown, corev1.ConditionUnknown, "", "msg1")},
		{watch.Modified, wait.CreateTestServiceWithConditions(name, corev1.ConditionUnknown, corev1.ConditionTrue, "", "msg2")},
		{watch.Modified, wait.CreateTestServiceWithConditions(name, corev1.ConditionTrue, corev1.ConditionTrue, "", "")},
	}
}

func TestServiceCreateImage(t *testing.T) {
	action, created, output, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "--async"}, false, false)
	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}
	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	} else if template.Spec.Containers[0].Image != "gcr.io/foo/bar:baz" {
		t.Fatalf("wrong image set: %v", template.Spec.Containers[0].Image)
	} else if !strings.Contains(output, "foo") || !strings.Contains(output, "created") ||
		!strings.Contains(output, commands.FakeNamespace) {
		t.Fatalf("wrong stdout message: %v", output)
	}
}

func TestServiceCreateImageSync(t *testing.T) {
	action, created, output, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz"}, false, true)
	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}
	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	}
	if template.Spec.Containers[0].Image != "gcr.io/foo/bar:baz" {
		t.Fatalf("wrong image set: %v", template.Spec.Containers[0].Image)
	}
	if !strings.Contains(output, "foo") || !strings.Contains(output, "Creating") ||
		!strings.Contains(output, commands.FakeNamespace) {
		t.Fatalf("wrong stdout message: %v", output)
	}
	if !strings.Contains(output, "Ready") {
		t.Fatalf("not running in sync mode")
	}
}

func TestServiceCreateImagePullPolicy(t *testing.T) {
	action, created, output, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "--async", "--pull-policy", "IfNotPresent"}, false, false)
	fmt.Printf(output)
	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}
	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	} else if template.Spec.Containers[0].ImagePullPolicy != "IfNotPresent" {
		t.Fatalf("wrong image pull policy: %v", template.Spec.Containers[0].ImagePullPolicy)
	}
}

func TestServiceCreateEnv(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "-e", "A=DOGS", "--env", "B=WOLVES", "--env=EMPTY", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedEnvVars := map[string]string{
		"A":     "DOGS",
		"B":     "WOLVES",
		"EMPTY": "",
	}

	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	}
	actualEnvVars, err := servinglib.EnvToMap(template.Spec.Containers[0].Env)
	if err != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	} else if template.Spec.Containers[0].Image != "gcr.io/foo/bar:baz" {
		t.Fatalf("wrong image set: %v", template.Spec.Containers[0].Image)
	} else if !reflect.DeepEqual(
		actualEnvVars,
		expectedEnvVars) {
		t.Fatalf("wrong env vars %v", template.Spec.Containers[0].Env)
	}
}

func TestServiceCreateWithRequests(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "--requests-cpu", "250m", "--requests-memory", "64Mi", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedRequestsVars := corev1.ResourceList{
		corev1.ResourceCPU:    parseQuantity(t, "250m"),
		corev1.ResourceMemory: parseQuantity(t, "64Mi"),
	}

	template, err := servinglib.RevisionTemplateOfService(created)

	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(
		template.Spec.Containers[0].Resources.Requests,
		expectedRequestsVars) {
		t.Fatalf("wrong requests vars %v", template.Spec.Containers[0].Resources.Requests)
	}
}

func TestServiceCreateWithLimits(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "--limits-cpu", "1000m", "--limits-memory", "1024Mi", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedLimitsVars := corev1.ResourceList{
		corev1.ResourceCPU:    parseQuantity(t, "1000m"),
		corev1.ResourceMemory: parseQuantity(t, "1024Mi"),
	}

	template, err := servinglib.RevisionTemplateOfService(created)

	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(
		template.Spec.Containers[0].Resources.Limits,
		expectedLimitsVars) {
		t.Fatalf("wrong limits vars %v", template.Spec.Containers[0].Resources.Limits)
	}
}

func TestServiceCreateRequestsLimitsCPU(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "--requests-cpu", "250m", "--limits-cpu", "1000m", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedRequestsVars := corev1.ResourceList{
		corev1.ResourceCPU: parseQuantity(t, "250m"),
	}

	expectedLimitsVars := corev1.ResourceList{
		corev1.ResourceCPU: parseQuantity(t, "1000m"),
	}

	template, err := servinglib.RevisionTemplateOfService(created)

	if err != nil {
		t.Fatal(err)
	} else {
		if !reflect.DeepEqual(
			template.Spec.Containers[0].Resources.Requests,
			expectedRequestsVars) {
			t.Fatalf("wrong requests vars %v", template.Spec.Containers[0].Resources.Requests)
		}

		if !reflect.DeepEqual(
			template.Spec.Containers[0].Resources.Limits,
			expectedLimitsVars) {
			t.Fatalf("wrong limits vars %v", template.Spec.Containers[0].Resources.Limits)
		}
	}
}

func TestServiceCreateRequestsLimitsMemory(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz", "--requests-memory", "64Mi", "--limits-memory", "1024Mi", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedRequestsVars := corev1.ResourceList{
		corev1.ResourceMemory: parseQuantity(t, "64Mi"),
	}

	expectedLimitsVars := corev1.ResourceList{
		corev1.ResourceMemory: parseQuantity(t, "1024Mi"),
	}

	template, err := servinglib.RevisionTemplateOfService(created)

	if err != nil {
		t.Fatal(err)
	} else {
		if !reflect.DeepEqual(
			template.Spec.Containers[0].Resources.Requests,
			expectedRequestsVars) {
			t.Fatalf("wrong requests vars %v", template.Spec.Containers[0].Resources.Requests)
		}

		if !reflect.DeepEqual(
			template.Spec.Containers[0].Resources.Limits,
			expectedLimitsVars) {
			t.Fatalf("wrong limits vars %v", template.Spec.Containers[0].Resources.Limits)
		}
	}
}

func TestServiceCreateMaxMinScale(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz",
		"--min-scale", "1", "--max-scale", "5", "--concurrency-target", "10", "--concurrency-limit", "100", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	}

	actualAnnos := template.Annotations
	expectedAnnos := []string{
		"autoscaling.knative.dev/minScale", "1",
		"autoscaling.knative.dev/maxScale", "5",
		"autoscaling.knative.dev/target", "10",
	}

	for i := 0; i < len(expectedAnnos); i += 2 {
		anno := expectedAnnos[i]
		if actualAnnos[anno] != expectedAnnos[i+1] {
			t.Fatalf("Unexpected annotation value for %s : %s (actual) != %s (expected)",
				anno, actualAnnos[anno], expectedAnnos[i+1])
		}
	}

	if *template.Spec.ContainerConcurrency != int64(100) {
		t.Fatalf("container concurrency not set to given value 1000")
	}
}

func TestServiceCreateRequestsLimitsCPUMemory(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz",
		"--requests-cpu", "250m", "--limits-cpu", "1000m",
		"--requests-memory", "64Mi", "--limits-memory", "1024Mi", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedRequestsVars := corev1.ResourceList{
		corev1.ResourceCPU:    parseQuantity(t, "250m"),
		corev1.ResourceMemory: parseQuantity(t, "64Mi"),
	}

	expectedLimitsVars := corev1.ResourceList{
		corev1.ResourceCPU:    parseQuantity(t, "1000m"),
		corev1.ResourceMemory: parseQuantity(t, "1024Mi"),
	}

	template, err := servinglib.RevisionTemplateOfService(created)

	if err != nil {
		t.Fatal(err)
	} else {
		if !reflect.DeepEqual(
			template.Spec.Containers[0].Resources.Requests,
			expectedRequestsVars) {
			t.Fatalf("wrong requests vars %v", template.Spec.Containers[0].Resources.Requests)
		}

		if !reflect.DeepEqual(
			template.Spec.Containers[0].Resources.Limits,
			expectedLimitsVars) {
			t.Fatalf("wrong limits vars %v", template.Spec.Containers[0].Resources.Limits)
		}
	}
}

func parseQuantity(t *testing.T, quantityString string) resource.Quantity {
	quantity, err := resource.ParseQuantity(quantityString)
	if err != nil {
		t.Fatal(err)
	}
	return quantity
}

func TestServiceCreateImageExistsAndNoForce(t *testing.T) {
	_, _, output, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:v2", "--async"}, true, false)
	if err == nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "foo") ||
		!strings.Contains(output, commands.FakeNamespace) ||
		!strings.Contains(output, "create") ||
		!strings.Contains(output, "--force") {
		t.Errorf("Invalid error output: '%s'", output)
	}

}

func TestServiceCreateImageForce(t *testing.T) {
	action, created, output, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--force", "--image", "gcr.io/foo/bar:v2", "--async"}, true, false)
	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("update", "services") {
		t.Fatalf("Bad action %v", action)
	}
	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	} else if template.Spec.Containers[0].Image != "gcr.io/foo/bar:v2" {
		t.Fatalf("wrong image set: %v", template.Spec.Containers[0].Image)
	} else if !strings.Contains(output, "foo") || !strings.Contains(output, commands.FakeNamespace) {
		t.Fatalf("wrong output: %s", output)
	}
}

func TestServiceCreateEnvForce(t *testing.T) {
	_, _, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:v1", "-e", "A=DOGS", "--env", "B=WOLVES", "--async"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	action, created, output, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--force", "--image", "gcr.io/foo/bar:v2", "-e", "A=CATS", "--env", "B=LIONS", "--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	expectedEnvVars := map[string]string{
		"A": "CATS",
		"B": "LIONS"}

	template, err := servinglib.RevisionTemplateOfService(created)
	if err != nil {
		t.Fatal(err)
	}
	actualEnvVars, err := servinglib.EnvToMap(template.Spec.Containers[0].Env)
	if err != nil {
		t.Fatal(err)
	} else if template.Spec.Containers[0].Image != "gcr.io/foo/bar:v2" {
		t.Fatalf("wrong image set: %v", template.Spec.Containers[0].Image)
	} else if !reflect.DeepEqual(
		actualEnvVars,
		expectedEnvVars) {
		t.Fatalf("wrong env vars:%v", template.Spec.Containers[0].Env)
	} else if !strings.Contains(output, "foo") || !strings.Contains(output, commands.FakeNamespace) {
		t.Fatalf("wrong output: %s", output)
	}
}

func TestServiceCreateWithServiceAccountName(t *testing.T) {
	action, created, _, err := fakeServiceCreate([]string{
		"service", "create", "foo", "--image", "gcr.io/foo/bar:baz",
		"--service-account", "foo-bar-account",
		"--async"}, false, false)

	if err != nil {
		t.Fatal(err)
	} else if !action.Matches("create", "services") {
		t.Fatalf("Bad action %v", action)
	}

	template, err := servinglib.RevisionTemplateOfService(created)

	if err != nil {
		t.Fatal(err)
	} else if template.Spec.ServiceAccountName != "foo-bar-account" {
		t.Fatalf("wrong service account name:%v", template.Spec.ServiceAccountName)
	}
}
